import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useState,
} from "react";
import { useNavigate } from "react-router";
import { AuthType, type MeResponse } from "@/gen/elara/auth/v1/auth_service_pb";
import {
	getAuthInfo,
	logout as logoutRpc,
	me,
} from "@/gen/elara/auth/v1/auth_service-AuthService_connectquery";

export interface AuthContextType {
	me: MeResponse | null;
	authType: AuthType;
	isLoading: boolean;
	logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

function AuthProviderInner({ children }: { children: React.ReactNode }) {
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const [isInitialLoading, setIsInitialLoading] = useState(true);

	const { data: authInfo, isLoading: isAuthInfoLoading } =
		useQuery(getAuthInfo);
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

	const logoutMutation = useMutation(logoutRpc);

	const logout = useCallback(async () => {
		try {
			await logoutMutation.mutateAsync({});
		} finally {
			queryClient.clear();
			navigate("/login", { replace: true });
		}
	}, [logoutMutation, queryClient, navigate]);

	useEffect(() => {
		if (isAuthInfoLoading) return;

		if (authType === AuthType.UNSPECIFIED) {
			return;
		}

		if (!isMeLoading) {
			if (meError) {
				setIsInitialLoading(false);
			} else if (meData) {
				if (meData.passwordChangeRequired) {
					navigate("/change-password");
				}
				setIsInitialLoading(false);
			}
		}
	}, [isAuthInfoLoading, isMeLoading, meData, meError, authType, navigate]);

	const value = useMemo(
		() => ({
			me: meData ?? null,
			authType,
			isLoading: isInitialLoading,
			logout,
		}),
		[meData, authType, isInitialLoading, logout],
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
