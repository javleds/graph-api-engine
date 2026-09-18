package domain

type Principal struct {
	OwnerID     string
	Permissions []string
}

func (p Principal) Can(permission string) bool {
	for _, v := range p.Permissions {
		if v == permission {
			return true
		}
	}
	return false
}
