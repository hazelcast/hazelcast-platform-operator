package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type hotbackupValidator struct {
	fieldValidator
}

func NewHotBackupValidator(o client.Object) hotbackupValidator {
	return hotbackupValidator{NewFieldValidator(o)}
}

func ValidateHotBackupPersistence(hb *HotBackup, h *Hazelcast) error {
	v := NewHotBackupValidator(hb)
	v.validateHotBackupPersistence(h)
	return v.Err()
}

func (v *hotbackupValidator) validateHotBackupPersistence(h *Hazelcast) {
	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		v.InternalError(Path("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
		return
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
		return
	}

	if !lastSpec.Persistence.IsEnabled() {
		v.Invalid(Path("spec", "persistenceEnabled"), lastSpec.Persistence.IsEnabled(), fmt.Sprintf("Hazelcast '%s' must enable persistence", h.Name))
		return
	}
}

func ValidateHotBackupIsNotReferencedByHazelcast(hb *HotBackup) error {
	v := NewHotBackupValidator(hb)

	hzList := HazelcastList{}
	err := kubeclient.List(context.Background(), &hzList, &client.ListOptions{Namespace: hb.Namespace})
	if err != nil {
		hotbackuplog.Error(err, "error on listing Hazelcast resources")
		return nil
	}

	for _, hz := range hzList.Items {
		hzCopy := hz
		v.validateHotBackupRestoreReference(&hzCopy, hb)
	}

	return v.Err()
}

func (v *hotbackupValidator) validateHotBackupRestoreReference(h *Hazelcast, hb *HotBackup) {
	if h.Spec.Persistence.RestoreFromHotBackupResourceName() {
		if *h.Spec.Persistence.Restore.HotBackupResourceName == hb.Name && h.DeletionTimestamp == nil {
			v.Forbidden(Path("spec", "persistence", "restore", "hotBackupResourceName"), fmt.Sprintf("Hazelcast '%s' has a restore reference to the Hotbackup", h.Name))
		}
	}
}
