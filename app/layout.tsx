import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_SITE_URL ?? "https://meridian-traffic-field.algo2147483647.chatgpt.site"),
  title: "Meridian City Traffic Field",
  description: "A live directed-graph traffic simulation with lane-aware movement and inspectable transforms.",
  openGraph: {
    title: "Meridian City Traffic Field",
    description: "Watch people find roads, join lanes, and react to traffic in a living city graph.",
    type: "website",
    images: [{ url: "/og.png", width: 1200, height: 630, alt: "Meridian City — Live Traffic Field" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Meridian City Traffic Field",
    description: "A live directed-graph traffic simulation.",
    images: ["/og.png"],
  },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
