package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	scalewayAPIBase = "https://api.scaleway.com"
	scrapeInterval  = 1 * time.Hour
	// Number of trailing UTC days to query as separate 1-day windows. Scaleway's
	// footprint pipeline lags by several days (verified 2026-05-03: ~3-4d) and
	// occasionally backfills, so we re-query a rolling window every scrape.
	lookbackDays = 14
)

// Response shape from /environmental-footprint/v1alpha1/data/query
// (verified against a real call on 2026-04-26).

type Impact struct {
	KgCO2Equivalent float64 `json:"kg_co2_equivalent"`
	M3WaterUsage    float64 `json:"m3_water_usage"`
}

type SKU struct {
	SKU             string `json:"sku"`
	ServiceCategory string `json:"service_category"`
	ProductCategory string `json:"product_category"`
	TotalSKUImpact  Impact `json:"total_sku_impact"`
}

type Zone struct {
	Zone            string `json:"zone"`
	TotalZoneImpact Impact `json:"total_zone_impact"`
	SKUs            []SKU  `json:"skus"`
}

type Region struct {
	Region            string `json:"region"`
	TotalRegionImpact Impact `json:"total_region_impact"`
	Zones             []Zone `json:"zones"`
}

type Project struct {
	ProjectID          string   `json:"project_id"`
	TotalProjectImpact Impact   `json:"total_project_impact"`
	Regions            []Region `json:"regions"`
}

type FootprintResponse struct {
	StartDate   string    `json:"start_date"`
	EndDate     string    `json:"end_date"`
	TotalImpact *Impact   `json:"total_impact"`
	Projects    []Project `json:"projects"`
}

type ProjectListResponse struct {
	Projects []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"projects"`
}

var (
	co2Gauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keit_scaleway_co2_kg",
			Help: "kgCO2e from the Scaleway Environmental Footprint API for a single UTC day, aggregated at SKU level (resource type, not per-instance). The report_date label is the UTC start of the 1-day window the value covers.",
		},
		[]string{"report_date", "project_id", "project_name", "region", "zone", "sku", "service_category", "product_category"},
	)
	waterGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keit_scaleway_water_m3",
			Help: "Water usage in m^3 from the Scaleway Environmental Footprint API for a single UTC day, aggregated at SKU level. Same labels as keit_scaleway_co2_kg.",
		},
		[]string{"report_date", "project_id", "project_name", "region", "zone", "sku", "service_category", "product_category"},
	)
	dataLagDaysGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "keit_scaleway_data_lag_days",
			Help: "Whole UTC days between the freshest possible report (yesterday) and the most recent day with non-empty Scaleway data within the lookback window. 0 means yesterday is available; lookbackDays means no data found in the window.",
		},
	)
	scrapeSuccessGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "keit_scaleway_scrape_success",
			Help: "1 if the most recent scrape successfully fetched at least one day of data, else 0. When 0, the per-day gauges retain their last-known-good values.",
		},
	)
)

func init() {
	prometheus.MustRegister(co2Gauge, waterGauge, dataLagDaysGauge, scrapeSuccessGauge)
}

type client struct {
	orgID      string
	token      string
	http       *http.Client
	projectsMu sync.RWMutex
	projects   map[string]string
}

func newClient(orgID, token string) *client {
	return &client{
		orgID:    orgID,
		token:    token,
		http:     &http.Client{Timeout: 30 * time.Second},
		projects: make(map[string]string),
	}
}

func (c *client) get(ctx context.Context, path string, q url.Values, out interface{}) error {
	u := scalewayAPIBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("scaleway %s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}

func (c *client) refreshProjectNames(ctx context.Context) error {
	q := url.Values{}
	q.Set("organization_id", c.orgID)
	q.Set("page_size", "100")
	var resp ProjectListResponse
	if err := c.get(ctx, "/account/v3/projects", q, &resp); err != nil {
		return err
	}
	m := make(map[string]string, len(resp.Projects))
	for _, p := range resp.Projects {
		m[p.ID] = p.Name
	}
	c.projectsMu.Lock()
	c.projects = m
	c.projectsMu.Unlock()
	return nil
}

func (c *client) projectName(id string) string {
	c.projectsMu.RLock()
	defer c.projectsMu.RUnlock()
	if name, ok := c.projects[id]; ok {
		return name
	}
	return id
}

func (c *client) fetchFootprint(ctx context.Context, start, end time.Time) (*FootprintResponse, error) {
	q := url.Values{}
	q.Set("organization_id", c.orgID)
	q.Set("start_date", start.UTC().Format(time.RFC3339))
	q.Set("end_date", end.UTC().Format(time.RFC3339))
	var resp FootprintResponse
	if err := c.get(ctx, "/environmental-footprint/v1alpha1/data/query", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type skuRecord struct {
	projectID, projectName             string
	region, zone, sku                  string
	serviceCategory, productCategory   string
	co2Kg, waterM3                     float64
}

type dayResult struct {
	reportDate string
	start      time.Time
	skus       []skuRecord
	fetched    bool
}

func (c *client) scrape(ctx context.Context) error {
	if err := c.refreshProjectNames(ctx); err != nil {
		log.Printf("project-name refresh failed (will use UUIDs as labels): %v", err)
	}

	now := time.Now().UTC()
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	results := make([]dayResult, 0, lookbackDays)
	successes := 0
	var mostRecentWithData time.Time

	for d := 0; d < lookbackDays; d++ {
		end := todayMidnight.AddDate(0, 0, -d)
		start := end.AddDate(0, 0, -1)
		reportDate := start.Format("2006-01-02")

		resp, err := c.fetchFootprint(ctx, start, end)
		if err != nil {
			log.Printf("fetch %s failed: %v", reportDate, err)
			results = append(results, dayResult{reportDate: reportDate, start: start, fetched: false})
			continue
		}
		successes++

		var skus []skuRecord
		for _, p := range resp.Projects {
			name := c.projectName(p.ProjectID)
			for _, r := range p.Regions {
				for _, z := range r.Zones {
					for _, s := range z.SKUs {
						skus = append(skus, skuRecord{
							projectID:       p.ProjectID,
							projectName:     name,
							region:          r.Region,
							zone:            z.Zone,
							sku:             s.SKU,
							serviceCategory: s.ServiceCategory,
							productCategory: s.ProductCategory,
							co2Kg:           s.TotalSKUImpact.KgCO2Equivalent,
							waterM3:         s.TotalSKUImpact.M3WaterUsage,
						})
					}
				}
			}
		}
		if len(skus) > 0 && start.After(mostRecentWithData) {
			mostRecentWithData = start
		}
		results = append(results, dayResult{reportDate: reportDate, start: start, skus: skus, fetched: true})
	}

	if successes == 0 {
		scrapeSuccessGauge.Set(0)
		log.Printf("scrape: all %d fetches failed; keeping last-known-good gauges", lookbackDays)
		return fmt.Errorf("all fetches failed")
	}

	co2Gauge.Reset()
	waterGauge.Reset()
	skuRowsTotal := 0
	daysWithData := 0
	for _, dr := range results {
		if !dr.fetched {
			continue
		}
		if len(dr.skus) > 0 {
			daysWithData++
		}
		for _, s := range dr.skus {
			labels := prometheus.Labels{
				"report_date":      dr.reportDate,
				"project_id":       s.projectID,
				"project_name":     s.projectName,
				"region":           s.region,
				"zone":             s.zone,
				"sku":              s.sku,
				"service_category": s.serviceCategory,
				"product_category": s.productCategory,
			}
			co2Gauge.With(labels).Set(s.co2Kg)
			waterGauge.With(labels).Set(s.waterM3)
			skuRowsTotal++
		}
	}

	freshestPossible := todayMidnight.AddDate(0, 0, -1)
	if !mostRecentWithData.IsZero() {
		lag := freshestPossible.Sub(mostRecentWithData).Hours() / 24
		dataLagDaysGauge.Set(lag)
	} else {
		dataLagDaysGauge.Set(float64(lookbackDays))
	}
	scrapeSuccessGauge.Set(1)

	freshestStr := "(none)"
	if !mostRecentWithData.IsZero() {
		freshestStr = mostRecentWithData.Format("2006-01-02")
	}
	log.Printf("scrape ok: %d/%d days fetched, %d days with data, %d SKU rows, freshest=%s",
		successes, lookbackDays, daysWithData, skuRowsTotal, freshestStr)
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func main() {
	fmt.Printf("Starting scaleway-footprint-exporter\n")

	orgID := mustEnv("SCW_ORG_ID")
	token := mustEnv("SCW_SECRET_KEY")

	c := newClient(orgID, token)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	ctx := context.Background()
	for {
		if err := c.scrape(ctx); err != nil {
			log.Printf("scrape failed: %v", err)
		}
		time.Sleep(scrapeInterval)
	}
}
