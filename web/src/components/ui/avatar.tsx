import type * as React from "react";
import { cn } from "@/lib/utils";

function Avatar({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="avatar"
			className={cn(
				"relative flex h-8 w-8 shrink-0 overflow-hidden rounded-full",
				className,
			)}
			{...props}
		/>
	);
}

function AvatarImage({ className, ...props }: React.ComponentProps<"img">) {
	return (
		<img
			data-slot="avatar-image"
			className={cn("aspect-square h-full w-full", className)}
			alt=""
			{...props}
		/>
	);
}

function AvatarFallback({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="avatar-fallback"
			className={cn(
				"flex h-full w-full items-center justify-center rounded-full bg-muted text-xs font-medium",
				className,
			)}
			{...props}
		/>
	);
}

export { Avatar, AvatarImage, AvatarFallback };
