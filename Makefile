run:
	go run .

dry_run:
	go run . --dry-run

install_deps:
	go mod download
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
