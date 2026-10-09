package rbac

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"

	"github.com/ez4bk/ezboot/utils"
)

type Def struct {
	Name string
	Desc string
	Api  []*Api
}

func (d *Def) GetName() string {
	return d.Name
}

func (d *Def) GetDesc() string {
	return d.Desc
}

type Api struct {
	Method string
	Uri    string
}

func (a *Api) getMethod() string {
	return a.Method
}

func (a *Api) GetUri() string {
	return a.Uri
}

type Enforcer struct {
	*casbin.Enforcer

	defs    map[string]*Def
	loaders map[uintptr]PolicyLoader
}

func (e *Enforcer) ListDef() []*Def {
	ds := make([]*Def, 0, len(e.defs))
	for _, def := range e.defs {
		ds = append(ds, def)
	}
	return ds
}

func (e *Enforcer) GetSysDef(role string) (*Def, bool) {
	def, ok := e.defs[role]
	return def, ok
}

type PolicyLoader func(model.Model) []*Def

func (e *Enforcer) LoadSysPolicy(l PolicyLoader) error {
	if _, ok := e.loaders[utils.Ptr(l)]; !ok {
		e.loaders[utils.Ptr(l)] = l
		for _, def := range l(e.GetModel()) {
			e.defs[def.Name] = def
		}
	}

	return e.RebuildRoles()
}

func (e *Enforcer) ReloadPolicy() error {
	if err := e.LoadPolicy(); err != nil {
		return err
	}

	for _, l := range e.loaders {
		for _, def := range l(e.GetModel()) {
			e.defs[def.Name] = def
		}
	}

	return e.RebuildRoles()
}

func (e *Enforcer) RebuildRoles() error {
	return e.BuildRoleLinks()
}

func (e *Enforcer) GetRoleDefForUser(name string, domain ...string) ([]*Def, error) {
	roles, err := e.GetImplicitRolesForUser(name, domain...)
	if err != nil {
		return nil, err
	}

	var defs []*Def
	for _, role := range roles {
		defs = append(defs, e.defs[role])
	}
	return defs, nil
}

func (e *Enforcer) Enforce(sub, obj, act string) (bool, error) {
	return e.Enforcer.Enforce(sub, obj, act)
}

func (e *Enforcer) EnforceEx(sub, obj, act string) (bool, []string, error) {
	return e.Enforcer.EnforceEx(sub, obj, act)
}

func DefaultModel(a persist.Adapter) (*Enforcer, error) {
	m, err := model.NewModelFromString(_DefaultModelDef)
	if err != nil {
		return nil, err
	}
	return New(m, a)
}

func New(m model.Model, a persist.Adapter) (*Enforcer, error) {
	e, err := casbin.NewEnforcer(m, a)
	if err != nil {
		return nil, err
	}

	return &Enforcer{
		Enforcer: e,
		defs:     make(map[string]*Def),
		loaders:  make(map[uintptr]PolicyLoader),
	}, nil
}
