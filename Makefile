init-crd:
	kubectl create -f ./artifacts/crd.yaml

init-role:
	kubectl create -f ./example/role.yaml

deploy:
	kubectl apply -f ./example/configmap.yaml -f ./example/deployment.yaml

api-verify:
	./gen/verify-codegen.sh

api-update:
	./gen/update-codegen.sh

container:
	docker build --rm -t bluehoodie/crypt-controller .

publish: container
	docker push bluehoodie/crypt-controller
