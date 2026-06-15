"use client";

import { useState } from "react";
import { Loader2Icon, Trash2Icon } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

export function DeleteConfirmPopover({
  title,
  description,
  confirmLabel,
  disabled,
  children,
  onConfirm,
}: {
  title: string;
  description: string;
  confirmLabel: string;
  disabled?: boolean;
  children: React.ReactNode;
  onConfirm: () => Promise<boolean>;
}) {
  const [open, setOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const confirm = async () => {
    setDeleting(true);
    const deleted = await onConfirm();
    setDeleting(false);
    if (deleted) setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild disabled={disabled}>
        {children}
      </PopoverTrigger>
      <PopoverContent onClick={(event) => event.stopPropagation()}>
        <div className="space-y-4">
          <div className="space-y-1">
            <p className="text-sm font-medium">{title}</p>
            <p className="text-xs leading-5 text-muted-foreground">
              {description}
            </p>
          </div>
          <div className="flex items-center justify-between gap-2 border-t pt-3">
            <Button
              size="sm"
              variant="ghost"
              className="min-w-20"
              disabled={deleting}
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="destructive"
              className="min-w-28"
              disabled={deleting}
              onClick={confirm}
            >
              {deleting ? (
                <Loader2Icon className="size-3.5 animate-spin" />
              ) : (
                <Trash2Icon className="size-3.5" />
              )}
              {confirmLabel}
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
