Behavior github action: Build and Dockerize KEIT Go Application.
- Can only be run by locomundo and geurjas (admins)
- It runs only in branch (not in the main branch)

Process of building a new docker image:

1. Create a new branch
2. Start manual the githubaction - Build and Dockerize KEIT Go Application - for this branch. (only amdins)
3. This will update the VERSION file in this branch. New docker images is pushed to our ghcr with tag VERSION (one minor version higher)
4. create yourself a pull request of this branch to main.
5. Merge the pull request to main. (only admins can bypass the review).
