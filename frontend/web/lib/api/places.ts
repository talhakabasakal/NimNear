import { apiBaseUrl } from "./events";

export type PlaceRecord = {
  id: string;
  name: string;
  description: string;
  latitude: number;
  longitude: number;
  address: string;
  category: string;
  image_url: string;
};

export type NearbyPlaceRecord = PlaceRecord & {
  distance_meters: number;
};

type NearbyPlacesResponse = { data: NearbyPlaceRecord[] };
type PlaceResponse = { data: PlaceRecord };
type PlaceErrorPayload = { message?: string; error?: string };

export class PlacesApiError extends Error {
  constructor(public readonly status: number, message = "Places API returned " + status) {
    super(message);
    this.name = "PlacesApiError";
  }
}

async function throwPlacesApiError(response: Response): Promise<never> {
  const payload = (await response.json().catch(() => null)) as PlaceErrorPayload | null;
  throw new PlacesApiError(response.status, payload?.message ?? payload?.error ?? "Places API returned " + response.status);
}

export async function fetchNearbyPlaces(input: { latitude: number; longitude: number; radius?: number }): Promise<NearbyPlaceRecord[]> {
  const url = new URL("/api/v1/places/nearby", apiBaseUrl);
  url.searchParams.set("lat", String(input.latitude));
  url.searchParams.set("lng", String(input.longitude));
  if (input.radius !== undefined) url.searchParams.set("radius", String(input.radius));

  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) await throwPlacesApiError(response);
  return ((await response.json()) as NearbyPlacesResponse).data;
}

export async function fetchPlace(id: string): Promise<PlaceRecord> {
  const response = await fetch(new URL("/api/v1/places/" + encodeURIComponent(id), apiBaseUrl), { cache: "no-store" });
  if (!response.ok) await throwPlacesApiError(response);
  return ((await response.json()) as PlaceResponse).data;
}
