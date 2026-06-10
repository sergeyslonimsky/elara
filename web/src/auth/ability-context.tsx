import { createContextualCan } from "@casl/react";
import { createContext, type ReactNode, useContext } from "react";
import { useAuth } from "@/components/auth-provider";
import { type AppAbility, denyAllAbility } from "./ability";

// AbilityContext carries the current caller's CASL ability. It defaults to
// denyAllAbility so any component reading it outside an AbilityProvider (or
// before the `me` query resolves) safely sees no permissions rather than
// crashing.
export const AbilityContext = createContext<AppAbility>(denyAllAbility);

// AbilityProvider bridges the auth state into AbilityContext: the ability is
// derived once from useAuth and shared with every useAbility()/<Can> consumer
// below it, so components no longer null-check the auth-state union.
export function AbilityProvider({ children }: { children: ReactNode }) {
	const { state } = useAuth();
	const ability =
		state.status === "authenticated" ? state.ability : denyAllAbility;

	return (
		<AbilityContext.Provider value={ability}>
			{children}
		</AbilityContext.Provider>
	);
}

// useAbility returns the current ability. Always non-null (denyAllAbility when
// unauthenticated), so callers can write `ability.can(...)` directly. Use this
// for boolean checks in logic (disabled=, list filtering); use <Can> for
// declarative JSX show/hide.
export function useAbility(): AppAbility {
	return useContext(AbilityContext);
}

// Can is the declarative show/hide gate, e.g.
//   <Can I="write" a="Group">{() => <Button>New Group</Button>}</Can>
export const Can = createContextualCan(AbilityContext.Consumer);
