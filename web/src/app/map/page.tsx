import type { Metadata } from "next";
import MapPage from "./MapPage";

export const metadata: Metadata = { title: "Map" };

export default function Page() {
  return <MapPage />;
}
