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
	TotalImpact Impact    `json:"total_impact"`
	Projects    []Project `json:"projects"`
}

// Project list response (subset). Endpoint path is /account/v3/projects per
// Scaleway IAM conventions — verify on first run; if the endpoint is different
// the exporter still works, it just labels metrics with UUIDs instead of names.
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
			Help: "Daily kgCO2e from Scaleway Environmental Footprint API, aggregated at SKU level (resource type, not per-instance).",
		},
		[]string{"project_id", "project_name", "region", "zone", "sku", "service_category", "product_category"},
	)
	waterGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keit_scaleway_water_m3",
			Help: "Daily water usage in m^3 from Scaleway Environmental Footprint API, aggregated at SKU level.",
		},
		[]string{"project_id", "project_name", "region", "zone", "sku", "service_category", "product_category"},
	)
)

func init() {
	prometheus.MustRegister(co2Gauge, waterGauge)
}

type client struct {
	orgID      string
	token      string
	http       *http.Client
	projectsMu sync.RWMutex
	projects   map[string]string // project_id -> name
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

func (c *client) scrape(ctx context.Context) error {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := end.Add(-24 * time.Hour)

	if err := c.refreshProjectNames(ctx); err != nil {
		log.Printf("project-name refresh failed (will use UUIDs as labels): %v", err)
	}

	resp, err := c.fetchFootprint(ctx, start, end)
	if err != nil {
		return err
	}

	co2Gauge.Reset()
	waterGauge.Reset()

	skuCount := 0
	for _, p := range resp.Projects {
		name := c.projectName(p.ProjectID)
		for _, r := range p.Regions {
			for _, z := range r.Zones {
				for _, s := range z.SKUs {
					labels := prometheus.Labels{
						"project_id":       p.ProjectID,
						"project_name":     name,
						"region":           r.Region,
						"zone":             z.Zone,
						"sku":              s.SKU,
						"service_category": s.ServiceCategory,
						"product_category": s.ProductCategory,
					}
					co2Gauge.With(labels).Set(s.TotalSKUImpact.KgCO2Equivalent)
					waterGauge.With(labels).Set(s.TotalSKUImpact.M3WaterUsage)
					skuCount++
				}
			}
		}
	}

	log.Printf("scrape ok: window %s..%s, %d projects, %d SKU rows, total %.3f kgCO2e",
		start.Format("2006-01-02"), end.Format("2006-01-02"),
		len(resp.Projects), skuCount, resp.TotalImpact.KgCO2Equivalent)
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
