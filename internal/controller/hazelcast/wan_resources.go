package hazelcast

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func getMapsGroupByHazelcastName(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanReplication) (map[string][]hazelcastv1alpha1.Map, error) {
	hzClientMap := make(map[string][]hazelcastv1alpha1.Map)
	for _, resource := range wan.Spec.Resources {
		switch resource.Kind {
		case hazelcastv1alpha1.ResourceKindMap:
			m, err := getWanMap(ctx, c, types.NamespacedName{Name: resource.Name, Namespace: wan.Namespace})
			if err != nil {
				return nil, err
			}
			mapList, ok := hzClientMap[m.Spec.HazelcastResourceName]
			if !ok {
				hzClientMap[m.Spec.HazelcastResourceName] = []hazelcastv1alpha1.Map{*m}
			} else {
				hzClientMap[m.Spec.HazelcastResourceName] = append(mapList, *m)
			}
		case hazelcastv1alpha1.ResourceKindHZ:
			maps, err := getAllMapsInHazelcast(ctx, c, resource.Name, wan.Namespace)
			if err != nil {
				return nil, err
			}
			// If no map is present for the Hazelcast resource
			if len(maps) == 0 {
				continue
			}
			mapList, ok := hzClientMap[resource.Name]
			if !ok {
				hzClientMap[resource.Name] = maps
			} else {
				hzClientMap[resource.Name] = append(mapList, maps...)
			}
		}
	}
	for k, v := range hzClientMap {
		hzClientMap[k] = removeDuplicate(v)
	}

	return hzClientMap, nil
}

func removeDuplicate(mapList []hazelcastv1alpha1.Map) []hazelcastv1alpha1.Map {
	keySet := make(map[types.NamespacedName]struct{})
	list := []hazelcastv1alpha1.Map{}
	for _, item := range mapList {
		nsname := types.NamespacedName{Name: item.Name, Namespace: item.Namespace}
		if _, ok := keySet[nsname]; !ok {
			keySet[nsname] = struct{}{}
			list = append(list, item)
		}
	}
	return list
}

func getWanMap(ctx context.Context, c client.Client, lk types.NamespacedName) (*hazelcastv1alpha1.Map, error) {
	m := &hazelcastv1alpha1.Map{}
	if err := c.Get(ctx, lk, m); err != nil {
		return nil, fmt.Errorf("failed to get Map CR from WanReplication: %w", err)
	}

	return m, nil
}

func getAllMapsInHazelcast(ctx context.Context, c client.Client, hazelcastResourceName string, wanNamespace string) ([]hazelcastv1alpha1.Map, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": hazelcastResourceName}
	nsMatcher := client.InNamespace(wanNamespace)

	wrl := &hazelcastv1alpha1.MapList{}

	if err := c.List(ctx, wrl, fieldMatcher, nsMatcher); err != nil {
		return nil, fmt.Errorf("could not get Map resources dependent under given Hazelcast %w", err)
	}
	return wrl.Items, nil
}

func wanMapKey(hzName, mapName string) string {
	return hzName + "__" + mapName
}
