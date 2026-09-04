test-all:
	go test -timeout 5m ./...
push-homelab:
	cd twigg-web && task push && cd ../twigg-track && task push
push-to-prod:
	cd twigg-web && task push-to-prod && cd ../twigg-track && task push-to-prod
push-yolo:
	cd twigg-web && task push && task push-to-prod && cd ../twigg-track && task push && task push-to-prod