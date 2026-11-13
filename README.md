# review-compare

Sandbox for comparing AI code reviewers on identical PRs.
Each folder `martian-N/` holds the files touched by one public benchmark PR (Martian code-review benchmark) and their sibling files, at the PR's base commit.
Each PR applies that benchmark PR's exact diff inside its folder.

Source PRs are public open-source PRs (keycloak, sentry, grafana, discourse, cal.com); original licenses apply to their files.
