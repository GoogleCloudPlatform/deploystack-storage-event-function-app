BASEDIR = $(shell pwd)
PROJECT=scaler-attempt
BUCKET = $(PROJECT)-bucket
PORT_API=8080

env:
	gcloud config set project $(PROJECT)

bucket:
	gsutil mb gs://$(BUCKET)

clean.bucket:
	gsutil rm -rf gs://$(BUCKET)	
	gsutil rb gs://$(BUCKET)	

clean.files:
	-gsutil rm -rf gs://$(BUCKET)/uploads/*
	-gsutil rm -rf gs://$(BUCKET)/processed/*	

test.files: clean.files
	gsutil cp function/ColtReto.png gs://$(BUCKET)/uploads/ColtReto.png


test.duplicate: 
	gsutil cp function/ColtReto.png gs://$(BUCKET)/uploads/ColtReto.png	

cleanapi:
	-docker stop scaler-goapi
	-docker rm scaler-goapi

api: cleanapi
	cd app && docker build -t scaler-goapi .
	docker run --name scaler-goapi --expose $(PORT_API) \
	-p $(PORT_API):$(PORT_API)  -e PORT=$(PORT_API) -e BUCKET=$(BUCKET) \
	-e GOOGLE_APPLICATION_CREDENTIALS=creds/creds.json -d scaler-goapi	
	docker logs scaler-goapi	