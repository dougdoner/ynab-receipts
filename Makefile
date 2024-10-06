docker:
	docker build -t ynab-receipts .


run: docker
	docker run ynab-receipts