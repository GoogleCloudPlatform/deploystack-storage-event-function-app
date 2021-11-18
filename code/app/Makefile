BASEDIR = $(shell pwd)
PROJECT=scaler-attempt
BUCKET = $(PROJECT)-bucket
SAACCOUNT=$(PROJECT)-dev-sa
SAACCOUNTEMAIL=$(SAACCOUNT)@$(PROJECT).iam.gserviceaccount.com

env:
	gcloud config set project $(PROJECT)

serviceaccount: env
	@echo ~~~ Create service account for Development   
	-gcloud iam service-accounts create $(SAACCOUNT) \
    --description "A service account for development of a bingo game" \
    --display-name "Bingo App" --project $(PROJECT)
	@echo ~~~ Download key for service account. 
	-gcloud iam service-accounts keys create creds/creds.json \
  	--iam-account $(SAACCOUNTEMAIL)


perms: env
	gsutil iam ch 'serviceaccount:$(SAACCOUNTEMAIL):roles/storage.admin' gs://$(BUCKET)