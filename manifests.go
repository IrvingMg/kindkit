package kindkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// ApplyManifests applies multi-document Kubernetes YAML to the cluster
// using server-side apply.
func (c *Cluster) ApplyManifests(ctx context.Context, manifests []byte) error {
	cfg, err := c.RESTConfig()
	if err != nil {
		return fmt.Errorf("get REST config: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}

	mapper, err := newRESTMapper(disc)
	if err != nil {
		return fmt.Errorf("build REST mapper: %w", err)
	}

	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifests), 4096)
	decodingSerializer := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("decode YAML document: %w", err)
		}
		if rawObj.Raw == nil {
			continue
		}

		obj := &unstructured.Unstructured{}
		if _, _, err := decodingSerializer.Decode(rawObj.Raw, nil, obj); err != nil {
			return fmt.Errorf("decode to unstructured: %w", err)
		}
		if obj.GetKind() == "" {
			continue
		}

		dr, err := resourceClient(dynClient, mapper, obj)
		if isNoKindMatch(err) {
			var refreshErr error
			mapper, refreshErr = newRESTMapper(disc)
			if refreshErr != nil {
				return fmt.Errorf("refresh REST mapper: %w", refreshErr)
			}
			dr, err = resourceClient(dynClient, mapper, obj)
		}
		if err != nil {
			return fmt.Errorf("resolve resource for %s %q: %w",
				obj.GetKind(), obj.GetName(), err)
		}

		if _, err := dr.Patch(ctx, obj.GetName(), types.ApplyPatchType, rawObj.Raw, metav1.PatchOptions{
			FieldManager: "kindkit",
		}); err != nil {
			return fmt.Errorf("apply %s %q: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	return nil
}

// resourceClient resolves GVK to GVR and returns the appropriate
// dynamic client for the object's scope.
func resourceClient(dynClient dynamic.Interface, mapper meta.RESTMapper, obj *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	gvk := obj.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = "default"
		}
		return dynClient.Resource(mapping.Resource).Namespace(ns), nil
	}
	return dynClient.Resource(mapping.Resource), nil
}

func newRESTMapper(disc discovery.DiscoveryInterface) (meta.RESTMapper, error) {
	groupResources, err := restmapper.GetAPIGroupResources(disc)
	if err != nil {
		return nil, err
	}
	return restmapper.NewDiscoveryRESTMapper(groupResources), nil
}

func isNoKindMatch(err error) bool {
	if err == nil {
		return false
	}
	return errors.As(err, new(*meta.NoKindMatchError))
}
