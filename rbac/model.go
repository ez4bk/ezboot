package rbac

const _DefaultModelDef = `
# sub = accessed entity
# obj = accessed resource
# act = access method

[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, eft

[role_definition]
# 'g = a, b' equals to 'a extends b'
g = _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = (r.act == p.act && keyMatch3(r.obj, p.obj) && (p.sub == 'Role:Anonymous' || g(r.sub, p.sub))) || g(r.sub, 'Role:Superuser')
`
