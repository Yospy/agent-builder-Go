"use client";

import { MotionConfig } from "motion/react";
import { ThemeProvider } from "next-themes";

import { TooltipProvider } from "@/components/ui/tooltip";

// MotionConfig reducedMotion="user" disables transform/layout animations for
// users with prefers-reduced-motion (opacity fades remain, per Motion docs).
export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
    >
      <MotionConfig reducedMotion="user">
        <TooltipProvider delayDuration={300}>{children}</TooltipProvider>
      </MotionConfig>
    </ThemeProvider>
  );
}
