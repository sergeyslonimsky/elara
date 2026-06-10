import { ConnectError } from "@connectrpc/connect";
import {
	createConnectQueryKey,
	useMutation,
	useQuery,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { createContext, useCallback, useContext, useMemo } from "react";
import { type AppAbility, buildAbility } from "@/auth/ability";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { getAuthInfo } from "@/gen/elara/auth/v1/auth_service-AuthService_connectquery";
import type { MeResponse } from "@/gen/elara/profile/v1/profile_service_pb";
import {
	logout as logoutRpc,
	me,
} from "@/gen/elara/profile/v1/profile_service-ProfileService_connectquery";

export type AuthState =
	| { status: "loading" }
	| { status: "error"; error: ConnectError }
	| { status: "anonymous"; authType: AuthType }
	| {
			status: "authenticated";
			authType: AuthType;
			user: MeResponse;
			ability: AppAbility;
	  };

export interface AuthContextType {
	state: AuthState;
	logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

function AuthProviderInner({ children }: { children: React.ReactNode }) {
	const queryClient = useQueryClient();

	const {
		data: authInfo,
		isLoading: isAuthInfoLoading,
		error: authInfoError,
	} = useQuery(
		getAuthInfo,
		{},
		{ staleTime: Infinity, gcTime: Infinity, retry: 2 },
	);

	const authType = authInfo?.authType ?? AuthType.UNSPECIFIED;

	const {
		data: meData,
		isLoading: isMeLoading,
		error: meError,
	} = useQuery(
		me,
		{},
		{
			enabled: authType !== AuthType.UNSPECIFIED,
			retry: false,
		},
	);

	const { mutateAsync: mutateLogout } = useMutation(logoutRpc);

	const logout = useCallback(async () => {
		try {
			await mutateLogout({});
		} finally {
			queryClient.removeQueries({
				queryKey: createConnectQueryKey({ schema: me, cardinality: undefined }),
			});
		}
	}, [mutateLogout, queryClient]);

	const state: AuthState = useMemo(() => {
		if (isAuthInfoLoading) return { status: "loading" };
		if (authInfoError)
			return { status: "error", error: ConnectError.from(authInfoError) };
		if (authType === AuthType.UNSPECIFIED) return { status: "loading" };

		if (authType === AuthType.NONE) {
			if (meData)
				return {
					status: "authenticated",
					authType,
					user: meData,
					ability: buildAbility(meData.permissions),
				};
			if (meError)
				return { status: "error", error: ConnectError.from(meError) };
			return { status: "loading" };
		}

		// BASIC / OIDC
		if (meData)
			return {
				status: "authenticated",
				authType,
				user: meData,
				ability: buildAbility(meData.permissions),
			};
		if (meError) return { status: "anonymous", authType };
		if (isMeLoading) return { status: "loading" };
		return { status: "anonymous", authType };
	}, [
		isAuthInfoLoading,
		authInfoError,
		authType,
		meData,
		meError,
		isMeLoading,
	]);

	const value = useMemo(
		() => ({
			state,
			logout,
		}),
		[state, logout],
	);

	return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function AuthProvider({
	children,
	initialValue,
}: {
	children: React.ReactNode;
	initialValue?: AuthContextType;
}) {
	if (initialValue) {
		return (
			<AuthContext.Provider value={initialValue}>
				{children}
			</AuthContext.Provider>
		);
	}

	return <AuthProviderInner>{children}</AuthProviderInner>;
}

export function useAuth() {
	const context = useContext(AuthContext);
	if (!context) {
		throw new Error("useAuth must be used within AuthProvider");
	}
	return context;
}
