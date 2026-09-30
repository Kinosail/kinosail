import { expect, test } from "@playwright/test";
import { registerHtmxJourneys } from "../../player/e2e/htmx-journeys";

registerHtmxJourneys("subtitles", test, expect);
