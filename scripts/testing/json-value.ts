export type JSONValue = string | number | boolean | null | JSONValue[] | { [key: string]: JSONValue | undefined };
export type JSONObject = { [key: string]: JSONValue | undefined };
