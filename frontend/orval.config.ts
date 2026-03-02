import { defineConfig } from "orval";

export default defineConfig({
  gateway: {
    input: {
      target: "../api/openapi/gateway.v0.yaml"
    },
    output: {
      target: "src/lib/api/generated/gateway.ts",
      schemas: "src/lib/api/generated/model",
      client: "fetch",
      mode: "split"
    }
  }
});
