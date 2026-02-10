# ResponsesCreateProfileCustomRulesBatchResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **str** |  | [optional] 
**created** | [**List[ResponsesCustomRuleBatchCreated]**](ResponsesCustomRuleBatchCreated.md) |  | [optional] 
**skipped** | [**List[ResponsesCustomRuleBatchSkipped]**](ResponsesCustomRuleBatchSkipped.md) |  | [optional] 
**total_requested** | **int** |  | [optional] 

## Example

```python
from moddns.models.responses_create_profile_custom_rules_batch_response import ResponsesCreateProfileCustomRulesBatchResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ResponsesCreateProfileCustomRulesBatchResponse from a JSON string
responses_create_profile_custom_rules_batch_response_instance = ResponsesCreateProfileCustomRulesBatchResponse.from_json(json)
# print the JSON string representation of the object
print(ResponsesCreateProfileCustomRulesBatchResponse.to_json())

# convert the object into a dict
responses_create_profile_custom_rules_batch_response_dict = responses_create_profile_custom_rules_batch_response_instance.to_dict()
# create an instance of ResponsesCreateProfileCustomRulesBatchResponse from a dict
responses_create_profile_custom_rules_batch_response_from_dict = ResponsesCreateProfileCustomRulesBatchResponse.from_dict(responses_create_profile_custom_rules_batch_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


