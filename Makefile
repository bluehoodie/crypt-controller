init-crd:
	kubectl create -f ./artifacts/crd.yaml

api-verify:
	./gen/verify-codegen.sh

api-update:
	./gen/update-codegen.sh

container:
	docker build --rm -t bluehoodie/crypt-controller .

publish: container
	docker push bluehoodie/crypt-controller
