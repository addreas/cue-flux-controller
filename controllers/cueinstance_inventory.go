/*
Copyright 2021 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"sort"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/ssa"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"

	cuev1alpha1 "github.com/phoban01/cue-flux-controller/api/v1alpha1"
)

func NewInventory() *cuev1alpha1.ResourceInventory {
	return &cuev1alpha1.ResourceInventory{
		Entries: []cuev1alpha1.ResourceRef{},
	}
}

// AddObjectsToInventory extracts the metadata from the given objects and adds it to the inventory.
func AddObjectsToInventory(inv *cuev1alpha1.ResourceInventory, set *ssa.ChangeSet) error {
	if set == nil {
		return nil
	}

	for _, entry := range set.Entries {
		inv.Entries = append(inv.Entries, cuev1alpha1.ResourceRef{
			ID:      entry.ObjMetadata.String(),
			Version: entry.GroupVersion,
		})
	}

	return nil
}

// ListObjectsInInventory returns the inventory entries as unstructured.Unstructured objects.
func ListObjectsInInventory(inv *cuev1alpha1.ResourceInventory) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)

	if inv.Entries == nil {
		return objects, nil
	}

	for _, entry := range inv.Entries {
		objMetadata, err := object.ParseObjMetadata(entry.ID)
		if err != nil {
			return nil, err
		}

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   objMetadata.GroupKind.Group,
			Kind:    objMetadata.GroupKind.Kind,
			Version: entry.Version,
		})
		u.SetName(objMetadata.Name)
		u.SetNamespace(objMetadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(ssa.SortableUnstructureds(objects))
	return objects, nil
}

// ListMetaInInventory returns the inventory entries as object.ObjMetadata objects.
func ListMetaInInventory(inv *cuev1alpha1.ResourceInventory) (object.ObjMetadataSet, error) {
	var metas []object.ObjMetadata
	for _, e := range inv.Entries {
		m, err := object.ParseObjMetadata(e.ID)
		if err != nil {
			return metas, err
		}
		metas = append(metas, m)
	}

	return metas, nil
}

// DiffInventory returns the slice of objects that do not exist in the target inventory.
func DiffInventory(inv *cuev1alpha1.ResourceInventory, target *cuev1alpha1.ResourceInventory) ([]*unstructured.Unstructured, error) {
	versionOf := func(i *cuev1alpha1.ResourceInventory, objMetadata object.ObjMetadata) string {
		for _, entry := range i.Entries {
			if entry.ID == objMetadata.String() {
				return entry.Version
			}
		}
		return ""
	}

	objects := make([]*unstructured.Unstructured, 0)
	aList, err := ListMetaInInventory(inv)
	if err != nil {
		return nil, err
	}

	bList, err := ListMetaInInventory(target)
	if err != nil {
		return nil, err
	}

	list := aList.Diff(bList)
	if len(list) == 0 {
		return objects, nil
	}

	for _, metadata := range list {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   metadata.GroupKind.Group,
			Kind:    metadata.GroupKind.Kind,
			Version: versionOf(inv, metadata),
		})
		u.SetName(metadata.Name)
		u.SetNamespace(metadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(ssa.SortableUnstructureds(objects))
	return objects, nil
}

func referenceToObjMetadataSet(cr []meta.NamespacedObjectKindReference) (object.ObjMetadataSet, error) {
	var objects []object.ObjMetadata

	for _, c := range cr {
		// For backwards compatibility with CueInstance v1beta1
		if c.APIVersion == "" {
			c.APIVersion = "apps/v1"
		}

		gv, err := schema.ParseGroupVersion(c.APIVersion)
		if err != nil {
			return objects, err
		}

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gv.Group,
			Kind:    c.Kind,
			Version: gv.Version,
		})
		u.SetName(c.Name)
		if c.Namespace != "" {
			u.SetNamespace(c.Namespace)
		}

		objects = append(objects, object.UnstructuredToObjMetadata(u))

	}

	return objects, nil
}
