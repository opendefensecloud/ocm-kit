apiserver:
  image:
    {{- $apiserver := index .resources "arc-apiserver-image" }}
    repository: {{ $apiserver.host }}/{{ $apiserver.repository }}
    tag: {{ $apiserver.tag }}

controller:
  image:
    {{- $controller := index .resources "arc-controller-manager-image" }}
    repository: {{ $controller.host }}/{{ $controller.repository }}
    tag: {{ $controller.tag }}

etcd:
  image:
    {{- $etcdImage := index .resources "etcd-image" }}
    repository: {{ $etcdImage.host }}/{{ $etcdImage.repository }}
    tag: {{ $etcdImage.tag }}
