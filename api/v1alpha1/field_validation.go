package v1alpha1

import "k8s.io/apimachinery/pkg/util/validation/field"

type fieldValidator field.ErrorList

func (v *fieldValidator) add(err ...*field.Error) {
	*v = append(*v, err...)
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
