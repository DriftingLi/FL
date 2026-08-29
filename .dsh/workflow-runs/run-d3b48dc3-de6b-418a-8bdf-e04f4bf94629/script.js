async function run(wf, args) {
  const found = await wf.phase("review", async () => {
    return await wf.parallel([
      async () => await wf.runAgent({
        name: "type-safety-audit",
        prompt: String(args.typeSafetyPrompt),
        readOnly: true,
        modelHint: "balanced",
        outputSchema: {
          type: "object",
          properties: {
            issues: { type: "array", items: { type: "object", properties: {
              severity: { type: "string", enum: ["critical","high","medium","low"] },
              file: { type: "string" }, lineRef: { type: "string" },
              description: { type: "string" }, suggestion: { type: "string" }
            }, required: ["severity","file","description"] } },
            overallRisk: { type: "string" }
          },
          required: ["issues","overallRisk"]
        }
      }),
      async () => await wf.runAgent({
        name: "contract-alignment",
        prompt: String(args.contractPrompt),
        readOnly: true,
        modelHint: "balanced",
        outputSchema: {
          type: "object",
          properties: {
            mismatches: { type: "array", items: { type: "object", properties: {
              severity: { type: "string", enum: ["critical","high","medium","low"] },
              frontend: { type: "string" }, backend: { type: "string" },
              description: { type: "string" }, fixHint: { type: "string" }
            }, required: ["severity","frontend","backend","description"] } },
            verdict: { type: "string" }
          },
          required: ["mismatches","verdict"]
        }
      }),
      async () => await wf.runAgent({
        name: "duplication-and-deadcode",
        prompt: String(args.dupPrompt),
        readOnly: true,
        modelHint: "balanced",
        outputSchema: {
          type: "object",
          properties: {
            deadCode: { type: "array", items: { type: "object", properties: { file: { type: "string" }, what: { type: "string" }, lineRef: { type: "string" } }, required: ["file","what"] } },
            duplicationLeft: { type: "array", items: { type: "object", properties: { file: { type: "string" }, what: { type: "string" } }, required: ["file","what"] } },
            unusedImports: { type: "array", items: { type: "string" } },
            summary: { type: "string" }
          },
          required: ["deadCode","duplicationLeft","unusedImports","summary"]
        }
      }),
      async () => await wf.runAgent({
        name: "template-binding-consistency",
        prompt: String(args.templatePrompt),
        readOnly: true,
        modelHint: "balanced",
        outputSchema: {
          type: "object",
          properties: {
            bindings: { type: "array", items: { type: "object", properties: { file: { type: "string" }, templateRef: { type: "string" }, defined: { type: "boolean" }, note: { type: "string" } }, required: ["file","templateRef","defined"] } },
            risks: { type: "array", items: { type: "string" } },
            verdict: { type: "string" }
          },
          required: ["bindings","risks","verdict"]
        }
      })
    ], { concurrency: 4 });
  });
  return found;
}

