// Package fakes provides hand-rolled test doubles for the domain Ports
// (UsersPort, GroupsPort, GroupRulesPort, PoliciesPort, LogsPort,
// RateLimitPort, HealthPort). Following CONVENTIONS §13.3 the template uses
// Func fields: a test wires exactly the methods it needs; any unwired method
// calls t.Fatalf so unexpected interactions fail loudly.
//
// Usage:
//
//	port := fakes.NewUsersPort(t)
//	port.GetFunc = func(ctx context.Context, id string) (domain.User, error) {
//	    return domain.User{ID: id}, nil
//	}
//	svc := service.NewUsersService(port)
//	_, err := svc.Get(ctx, "00u_active_alice")
//	require.NoError(t, err)
//
// testify/mock is allowed only when sequencing behaviour is hard to express
// with a Func field (CONVENTIONS §13.3).
package fakes
