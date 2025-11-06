

run-dev:
	infisical run --env=dev -- go run cmd/api/main.go

run-stage:
	infisical run --env=stage -- go run cmd/api/main.go

run-prod:
	infisical run --env=prod -- gor run cmd/api/main.go

up:
	@docker compose up -d
down:
	@docker compose down -v
