import { AbilityProvider as CaslAbilityProvider } from "@casl/react";
import { createContext, type ReactNode, useContext } from "react";
import { useAuth } from "@/components/auth-provider";
import { type AppAbility, denyAllAbility } from "./ability";

// Can is the declarative show/hide gate, e.g.
//   <Can I="write" a="Group">{() => <Button>New Group</Button>}</Can>
//
// In @casl/react v7 it reads from the library's own context rather than one we
// bind it to — createContextualCan was removed in that release. Note this
// means <Can> is NOT covered by the deny-by-default fallback below: rendered
// outside AbilityProvider it throws, where useAbility() would return
// denyAllAbility.
export { Can } from "@casl/react";

// AbilityContext exists alongside the library's context to preserve the
// deny-by-default guarantee. @casl/react v7's useAbility() throws when no
// provider is mounted ("AbilityContext is not provided"), because its context
// defaults to null. Elara wants the opposite: a component rendered outside a
// provider, or before the `me` query resolves, should see no permissions
// rather than crash the page.
//
// Reading our own context instead of try/catching theirs also avoids depending
// on the order in which their hook calls useSyncExternalStore relative to the
// throw — a detail of their build, not of their API.
const AbilityContext = createContext<AppAbility>(denyAllAbility);

// AbilityProvider bridges the auth state into both contexts: the ability is
// derived once from useAuth and shared with every useAbility()/<Can> consumer
// below it, so components no longer null-check the auth-state union.
export function AbilityProvider({ children }: { children: ReactNode }) {
	const { state } = useAuth();
	const ability =
		state.status === "authenticated" ? state.ability : denyAllAbility;

	return (
		<AbilityContext.Provider value={ability}>
			<CaslAbilityProvider value={ability}>{children}</CaslAbilityProvider>
		</AbilityContext.Provider>
	);
}

// useAbility returns the current ability. Always non-null (denyAllAbility when
// unauthenticated or outside a provider), so callers can write
// `ability.can(...)` directly. Use this for boolean checks in logic
// (disabled=, list filtering); use <Can> for declarative JSX show/hide.
//
// Abilities are immutable — buildAbility returns a fresh object and auth
// changes swap it wholesale — so plain context identity is enough to
// re-render. The library's useAbility additionally subscribes to a rule
// "updated" event, which nothing here emits.
export function useAbility(): AppAbility {
	return useContext(AbilityContext);
}
