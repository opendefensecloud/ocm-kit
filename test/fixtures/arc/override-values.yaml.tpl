foobar:
  image:
    {{- $apiserver := index .OCIResources "arc-apiserver-image" }}
    repository: {{ $apiserver.Host }}/{{ $apiserver.Repository }}
    tag: {{ $apiserver.Tag }}

fizzbuzz:
  image:
    {{- $controller := index .OCIResources "arc-controller-manager-image" }}
    repository: {{ $controller.Host }}/{{ $controller.Repository }}
    tag: {{ $controller.Tag }}

helloworld:
  image:
    {{- $etcdImage := index .OCIResources "etcd-image" }}
    repository: {{ $etcdImage.Host }}/{{ $etcdImage.Repository }}
    tag: {{ $etcdImage.Tag }}
