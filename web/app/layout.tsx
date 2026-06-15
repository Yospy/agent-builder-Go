import type { Metadata } from "next";
import { Roboto, Roboto_Mono } from "next/font/google";
import "./globals.css";

import { AppSidebar } from "@/components/sidebar/app-sidebar";
import { Providers } from "@/components/providers";
import { Toaster } from "@/components/ui/sonner";

const robotoSans = Roboto({
  variable: "--font-sans",
  subsets: ["latin"],
  weight: ["400", "500", "700"],
});

const robotoMono = Roboto_Mono({
  // globals.css maps --font-mono to this variable name; keeping it avoids
  // touching the generated theme block.
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Agent Builder",
  description: "Build and run agents — every agent is a row.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${robotoSans.variable} ${robotoMono.variable} h-full antialiased`}
    >
      <body className="flex h-dvh overflow-hidden bg-background text-foreground">
        <Providers>
          <AppSidebar />
          {children}
          <Toaster />
        </Providers>
      </body>
    </html>
  );
}
