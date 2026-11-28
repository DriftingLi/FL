package valuation

const valuationSuggestionSchemaJSON = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "additionalProperties":false,
  "required":["schema_version","patches","supplements","questions"],
  "properties":{
    "schema_version":{"type":"integer","const":1},
    "patches":{
      "type":"array","maxItems":18,
      "items":{
        "type":"object","additionalProperties":false,
        "required":["field","state","value","range","candidate_keys","evidence"],
        "properties":{
          "field":{"type":"string","enum":["brand","vehicle_type","series","tonnage","config_type","mast_type","mast_height_mm","factory_year","sale_year","usage_hours","original_paint","province","city","has_license_plate","has_registration_certificate","has_maintenance_records","condition_rating"]},
          "state":{"type":"string","enum":["missing","unknown","value","range","candidate"]},
          "value":{},
          "range":{"type":["object","null"],"additionalProperties":false,"required":["min","max"],
            "properties":{"min":{"type":"number"},"max":{"type":"number"}}
          },
          "candidate_keys":{"type":"array","maxItems":8,"uniqueItems":true,"items":{"type":"string","minLength":1,"maxLength":160}},
          "evidence":{"type":"array","maxItems":8,"items":{"type":"string","minLength":1,"maxLength":300}}
        }
      }
    },
    "supplements":{
      "type":"object","maxProperties":20,
      "propertyNames":{"pattern":"^[a-z][a-z0-9_]{0,31}$"},
      "additionalProperties":{"type":"string","maxLength":500}
    },
    "questions":{
      "type":"array","maxItems":5,"uniqueItems":true,
      "items":{"type":"string","minLength":1,"maxLength":300}
    }
  }
}`
