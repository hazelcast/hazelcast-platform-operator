package v1alpha1

import (
	"context"
	"encoding/json"
	"errors"
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

func ValidateHotBackupPersistence(h *Hazelcast) error {
	v := NewHotBackupValidator(h)
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
		v.Invalid(Path("spec", "persistenceEnabled"), lastSpec.Persistence.IsEnabled(), "Persistence must be enabled at Hazelcast")
		return
	}
}

func ValidateHotBackupIsNotReferencedByHazelcast(hb *HotBackup) error {
	hzList := HazelcastList{}

	err := kubeclient.List(context.Background(), &hzList, &client.ListOptions{Namespace: hb.Namespace})
	if err != nil {
		hotbackuplog.Error(err, "error on listing Hazelcast resources")
	}

	var errs []error
	for _, hz := range hzList.Items {
		hzCopy := hz
		errs = append(errs, ValidateHotBackupRestoreReference(&hzCopy, hb))
	}

	return joinErrors(errs)
}

func ValidateHotBackupRestoreReference(h *Hazelcast, hb *HotBackup) error {
	v := NewHotBackupValidator(h)
	v.validateHotBackupRestoreReference(h, hb)
	return v.Err()
}

func (v *hotbackupValidator) validateHotBackupRestoreReference(h *Hazelcast, hb *HotBackup) {
	if h.Spec.Persistence.IsEnabled() {
		if h.Spec.Persistence.Restore.HotBackupResourceName == hb.Name {
			v.Forbidden(Path("spec", "persistence", "restore", "hotBackupResourceName"), fmt.Sprintf("HotBackup '%s' is referenced by Hazelcast restore", hb.Name))
		}
	}
}

// It can be removed after we update Go to 1.20 or forward
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	var b []byte
	for i, err := range errs {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, err.Error()...)
	}
	return errors.New(string(b))
}
