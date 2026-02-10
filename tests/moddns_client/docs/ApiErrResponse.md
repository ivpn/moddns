# ApiErrResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**details** | **List[str]** |  | [optional] 
**error** | **str** |  | [optional] 

## Example

```python
from moddns.models.api_err_response import ApiErrResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ApiErrResponse from a JSON string
api_err_response_instance = ApiErrResponse.from_json(json)
# print the JSON string representation of the object
print(ApiErrResponse.to_json())

# convert the object into a dict
api_err_response_dict = api_err_response_instance.to_dict()
# create an instance of ApiErrResponse from a dict
api_err_response_from_dict = ApiErrResponse.from_dict(api_err_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


