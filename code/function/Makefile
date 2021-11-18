BASEDIR = $(shell pwd)
PROJECT = scaler-attempt
BUCKET = $(PROJECT)-bucket

env:
	gcloud config set project $(PROJECT)

deploy:
	gcloud functions deploy scaler --runtime go116 \
	--trigger-resource $(BUCKET) --trigger-event google.storage.object.finalize \
	--entry-point OnFileUpload
