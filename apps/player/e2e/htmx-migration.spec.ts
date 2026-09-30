import { expect, test } from "@playwright/test";
import { registerHtmxJourneys } from "./htmx-journeys";

registerHtmxJourneys("player", test, expect);
