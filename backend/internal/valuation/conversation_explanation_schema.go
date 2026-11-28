package valuation

const valuationExplanationSchemaJSON = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "additionalProperties":false,
  "required":["schema_version","action","template_key","evidence_ids"],
  "properties":{
    "schema_version":{"type":"integer","const":1},
    "action":{"type":"string","enum":["explain","recalculate"]},
    "template_key":{"type":"string","enum":["hours_coefficient","value_interval","vehicle_configuration","locked_formula","assumptions","condition_market"]},
    "evidence_ids":{"type":"array","maxItems":16,"uniqueItems":true,"items":{"type":"string","minLength":1,"maxLength":160}}
  }
}`
