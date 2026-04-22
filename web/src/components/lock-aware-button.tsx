import { Link } from "react-router";
import { Button } from "@/components/ui/button";

interface LockAwareButtonProps {
	/** Whether the action is blocked because something is locked. */
	locked: boolean;
	/** Tooltip shown when locked (e.g. `"Namespace \"prod\" is locked"`). */
	lockedReason?: string;
	/** Route to navigate to when unlocked. If absent, `onClick` is used. */
	to?: string;
	onClick?: () => void;
	children: React.ReactNode;
	/** Forwarded to Button. */
	variant?: React.ComponentProps<typeof Button>["variant"];
	size?: React.ComponentProps<typeof Button>["size"];
	className?: string;
}

/**
 * A button that renders as:
 *   - a disabled Button with tooltip when `locked` is true
 *   - a Link-rendered Button when `to` is set
 *   - a normal onClick Button otherwise
 *
 * Centralises the pattern: `{locked ? <Button disabled title/> : <Button render={<Link />}/>}`.
 */
export function LockAwareButton({
	locked,
	lockedReason,
	to,
	onClick,
	children,
	variant,
	size,
	className,
}: LockAwareButtonProps) {
	if (locked) {
		return (
			<Button
				variant={variant}
				size={size}
				className={className}
				disabled
				title={lockedReason}
			>
				{children}
			</Button>
		);
	}

	if (to) {
		return (
			<Button
				variant={variant}
				size={size}
				className={className}
				render={<Link to={to} />}
			>
				{children}
			</Button>
		);
	}

	return (
		<Button
			variant={variant}
			size={size}
			className={className}
			onClick={onClick}
		>
			{children}
		</Button>
	);
}
