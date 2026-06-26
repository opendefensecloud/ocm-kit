foobar:
  image:
    {{- $apiserver := index .OCIResources "arc-apiserver-image" }}
    repository: {{ $apiserver.Host }}/{{ $apiserver.Repository }}
    tag: {{ $apiserver.Tag }}
  imagePullSecrets:
    - name: {{ pullSecretFor $apiserver.Host }}

fizzbuzz:
  image:
    {{- $controller := index .OCIResources "arc-controller-manager-image" }}
    repository: {{ $controller.Host }}/{{ $controller.Repository }}
    tag: {{ $controller.Tag }}
  imagePullSecrets:
    - name: {{ pullSecretFor (printf "%s/%s" $controller.Host $controller.Repository) }}

helloworld:
  image:
    {{- $etcdImage := index .OCIResources "etcd-image" }}
    repository: {{ $etcdImage.Host }}/{{ $etcdImage.Repository }}
    tag: {{ $etcdImage.Tag }}
  imagePullSecrets:
    - name: {{ pullSecretFor (printf "%s/%s:%s" $etcdImage.Host $etcdImage.Repository $etcdImage.Tag) }}
