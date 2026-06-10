import { Code, ConnectError } from "@connectrpc/connect";
import { createConnectQueryKey, useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { LogIn } from "lucide-react";
import { useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import {
	basicLogin,
	oIDCLogin,
} from "@/gen/elara/auth/v1/auth_service-AuthService_connectquery";
import { me } from "@/gen/elara/profile/v1/profile_service-ProfileService_connectquery";

export function LoginPage() {
	const { state } = useAuth();
	const queryClient = useQueryClient();
	const [email, setEmail] = useState("");
	const [password, setPassword] = useState("");
	const [error, setError] = useState<string | null>(null);

	const basicLoginMutation = useMutation(basicLogin);
	const oidcLoginMutation = useMutation(oIDCLogin);

	const authType =
		state.status === "anonymous" ? state.authType : AuthType.UNSPECIFIED;

	const onBasicSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		setError(null);
		try {
			await basicLoginMutation.mutateAsync({
				email,
				password,
			});

			await queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({ schema: me, cardinality: "finite" }),
			});
		} catch (err) {
			const connectErr = ConnectError.from(err);
			if (connectErr.code === Code.Unauthenticated) {
				setError("Invalid email or password");
			} else {
				setError(connectErr.message || "An unexpected error occurred");
			}
		}
	};

	const onOIDCLogin = async () => {
		try {
			const response = await oidcLoginMutation.mutateAsync({});
			window.location.href = response.redirectUrl;
		} catch (err) {
			const connectErr = ConnectError.from(err);
			setError(connectErr.message || "Failed to initialize OIDC login");
		}
	};

	return (
		<div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
			<Card className="w-full max-w-sm">
				<CardHeader className="text-center">
					<CardTitle className="text-2xl font-bold uppercase tracking-tight">
						Elara
					</CardTitle>
					<CardDescription>
						{authType === AuthType.OIDC
							? "Sign in with your identity provider"
							: "Enter your credentials to access your account"}
					</CardDescription>
				</CardHeader>
				<CardContent>
					{authType === AuthType.OIDC && (
						<Button
							className="w-full"
							onClick={onOIDCLogin}
							disabled={oidcLoginMutation.isPending}
						>
							<LogIn className="mr-2 h-4 w-4" />
							Sign in with your identity provider
						</Button>
					)}

					{authType === AuthType.BASIC && (
						<form onSubmit={onBasicSubmit} className="space-y-4">
							<Field>
								<FieldLabel>Email</FieldLabel>
								<Input
									type="email"
									value={email}
									onChange={(e) => setEmail(e.target.value)}
									placeholder="name@example.com"
									autoComplete="email"
									required
								/>
							</Field>
							<Field>
								<FieldLabel>Password</FieldLabel>
								<Input
									type="password"
									value={password}
									onChange={(e) => setPassword(e.target.value)}
									autoComplete="current-password"
									required
								/>
							</Field>
							{error && (
								<div className="text-sm font-medium text-destructive">
									{error}
								</div>
							)}
							<Button
								type="submit"
								className="w-full"
								disabled={basicLoginMutation.isPending || !email || !password}
							>
								{basicLoginMutation.isPending ? "Signing in..." : "Sign in"}
							</Button>
						</form>
					)}
				</CardContent>
			</Card>
		</div>
	);
}
