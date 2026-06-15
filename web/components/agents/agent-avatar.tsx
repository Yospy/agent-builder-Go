import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";

export function AgentAvatar({
  name,
  className,
  inverted = false,
  filled = false,
}: {
  name: string;
  className?: string;
  // inverted: light fill for build mode's white-on-black header surface.
  inverted?: boolean;
  // filled: dark fill to mark the Builder on regular white surfaces.
  filled?: boolean;
}) {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase())
    .join("");

  return (
    <Avatar className={cn("border", className)}>
      <AvatarFallback
        className={cn(
          "text-xs font-medium",
          inverted && "bg-background text-foreground",
          filled && "bg-foreground text-background",
        )}
      >
        {initials || "?"}
      </AvatarFallback>
    </Avatar>
  );
}
