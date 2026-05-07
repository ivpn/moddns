# ResponsesDeletionCodeResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**code** | **str** |  | [optional] 
**expires_at** | **str** |  | [optional] 

## Example

```python
from moddns.models.responses_deletion_code_response import ResponsesDeletionCodeResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ResponsesDeletionCodeResponse from a JSON string
responses_deletion_code_response_instance = ResponsesDeletionCodeResponse.from_json(json)
# print the JSON string representation of the object
print(ResponsesDeletionCodeResponse.to_json())

# convert the object into a dict
responses_deletion_code_response_dict = responses_deletion_code_response_instance.to_dict()
# create an instance of ResponsesDeletionCodeResponse from a dict
responses_deletion_code_response_from_dict = ResponsesDeletionCodeResponse.from_dict(responses_deletion_code_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


