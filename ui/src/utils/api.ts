import createFetchClient from "openapi-fetch";
import createClient from "openapi-react-query";
import type { paths } from "@/lib/api";
import fetch from "./fetch-throw";

export const fetchClient = createFetchClient<paths>({
  baseUrl: "/api",
  fetch,
  headers: {
    "Content-Type": "application/json",
  },
  requestInitExt: {
    signal: AbortSignal.timeout(180 * 1000),
  },
});

export const $api = createClient(fetchClient);
