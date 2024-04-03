package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type fieldValidator struct {
	errs   field.ErrorList
	object client.Object
}

func NewFieldValidator(o client.Object) fieldValidator {
	return fieldValidator{
		object: o,
	}
}

func (v *fieldValidator) Err() error {
	if len(v.errs) != 0 {
		g := v.object.GetObjectKind().GroupVersionKind()
		return kerrors.NewInvalid(
			schema.GroupKind{
				Group: g.Group,
				Kind:  g.Kind,
			},
			v.object.GetName(),
			field.ErrorList(v.errs),
		)
	}
	return nil
}

func (v *fieldValidator) add(err ...*field.Error) {
	v.errs = append(v.errs, err...)
}

func (v *fieldValidator) Forbidden(p *field.Path, detail string) {
	v.add(field.Forbidden(p, detail))
}

func (v *fieldValidator) Invalid(p *field.Path, value interface{}, detail string) {
	v.add(field.Invalid(p, value, detail))
}

func (v *fieldValidator) NotFound(p *field.Path, value interface{}) {
	v.add(field.NotFound(p, value))
}

func (v *fieldValidator) Required(p *field.Path, detail string) {
	v.add(field.Required(p, detail))
}

func (v *fieldValidator) Duplicate(p *field.Path, value interface{}) {
	v.add(field.Duplicate(p, value))
}

func (v *fieldValidator) InternalError(p *field.Path, err error) {
	v.add(field.InternalError(p, err))
}

func Path(n string, m ...string) *field.Path {
	return field.NewPath(n, m...)
}
