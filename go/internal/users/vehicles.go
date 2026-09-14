package users

// MayRegisterVehicle reports whether a role may register a vehicle (PRD 010).
//
// Everyone except a spejder. Written as an exclusion rather than a list of the
// permitted roles, mirroring MayUseContacts above it — and for the reason
// config/roles.ts spells out on the client: an allow-list silently denies any role
// added later, and the newcomer would be an adult who drives to the event and
// whose car then stays out of the inventory.
//
// The exclusion itself is not about privilege. Spejdere are minors and do not
// drive to Nathejk as drivers, so there is nothing for the step to ask them
// (PRD 010 §4). This gates the endpoints; the client's matching check only decides
// whether to draw the form, since a hidden control is not access control.
func MayRegisterVehicle(viewer Role) bool {
	return viewer.Valid() && viewer != RoleSpejder
}
