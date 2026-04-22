import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { Client, ClientEvent } from "@/gen/elara/clients/v1/clients_pb";
import { ActivityTab } from "./activity-tab";
import { CountersTab } from "./counters-tab";
import { ErrorsTab } from "./errors-tab";
import { MethodDonut } from "./method-donut";
import { SessionsTab } from "./sessions-tab";
import { WatchesTab } from "./watches-tab";

interface DetailTabsProps {
	client: Client;
	events: ClientEvent[];
	isActive: boolean;
}

export function DetailTabs({ client, events, isActive }: DetailTabsProps) {
	const totalErrors = Number(client.errorCount);

	return (
		<Tabs defaultValue="activity">
			<TabsList>
				<TabsTrigger value="activity">Activity</TabsTrigger>
				<TabsTrigger value="counters">Counters</TabsTrigger>
				<TabsTrigger value="errors">
					Errors{" "}
					{totalErrors > 0 && (
						<span className="ml-1 text-destructive">({totalErrors})</span>
					)}
				</TabsTrigger>
				<TabsTrigger value="watches">Watches</TabsTrigger>
				<TabsTrigger value="sessions">Sessions</TabsTrigger>
			</TabsList>

			<TabsContent value="activity">
				<Card className="rounded-xl">
					<CardContent className="pt-4">
						<ActivityTab events={events} isActive={isActive} />
					</CardContent>
				</Card>
			</TabsContent>

			<TabsContent value="counters">
				<div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<Card className="rounded-xl">
						<CardHeader>
							<CardTitle className="text-sm">Requests by method</CardTitle>
						</CardHeader>
						<CardContent>
							<CountersTab client={client} />
						</CardContent>
					</Card>
					<Card className="rounded-xl">
						<CardHeader>
							<CardTitle className="text-sm">Method distribution</CardTitle>
						</CardHeader>
						<CardContent className="h-64">
							<MethodDonut client={client} />
						</CardContent>
					</Card>
				</div>
			</TabsContent>

			<TabsContent value="errors">
				<Card className="rounded-xl">
					<CardContent className="pt-4">
						<ErrorsTab
							events={events}
							totalErrors={totalErrors}
							isActive={isActive}
						/>
					</CardContent>
				</Card>
			</TabsContent>

			<TabsContent value="watches">
				<Card className="rounded-xl">
					<CardContent className="pt-6">
						<WatchesTab client={client} isActive={isActive} />
					</CardContent>
				</Card>
			</TabsContent>

			<TabsContent value="sessions">
				<Card className="rounded-xl">
					<CardContent className="pt-4">
						<SessionsTab client={client} />
					</CardContent>
				</Card>
			</TabsContent>
		</Tabs>
	);
}
