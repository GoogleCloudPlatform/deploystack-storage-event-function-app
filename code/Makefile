BASEDIR = $(shell pwd)
PROJECT=scaler-attempt
BUCKET = $(PROJECT)-bucket
PORT_API=8080

env:
	gcloud config set project $(PROJECT)

bucket:
	gcloud storage buckets create gs://$(BUCKET)

clean.bucket:
	gcloud storage rm --recursive --continue-on-error gs://$(BUCKET)	
	gcloud storage buckets delete gs://$(BUCKET)	

clean.files:
	-gcloud storage rm --recursive --continue-on-error gs://$(BUCKET)/uploads/*
	-gcloud storage rm --recursive --continue-on-error gs://$(BUCKET)/processed/*	

test.files: clean.files
	gcloud storage cp function/ColtReto.png gs://$(BUCKET)/uploads/ColtReto.png


test.duplicate: 
	gcloud storage cp function/ColtReto.png gs://$(BUCKET)/uploads/ColtReto.png	

cleanapi:
	-docker stop scaler-goapi
	-docker rm scaler-goapi

api: cleanapi
	cd app && docker build -t scaler-goapi .
	docker run --name scaler-goapi --expose $(PORT_API) \
	-p $(PORT_API):$(PORT_API)  -e PORT=$(PORT_API) -e BUCKET=$(BUCKET) \
	-e GOOGLE_APPLICATION_CREDENTIALS=creds/creds.json -d scaler-goapi	
	docker logs scaler-goapi	