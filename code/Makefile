BASEDIR = $(shell pwd)
PROJECT=scaler-attempt
BUCKET = $(PROJECT)-bucket

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