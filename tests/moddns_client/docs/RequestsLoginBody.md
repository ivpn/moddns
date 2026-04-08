# RequestsLoginBody


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | 
**password** | **str** |  | 

## Example

```python
from moddns.models.requests_login_body import RequestsLoginBody

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsLoginBody from a JSON string
requests_login_body_instance = RequestsLoginBody.from_json(json)
# print the JSON string representation of the object
print(RequestsLoginBody.to_json())

# convert the object into a dict
requests_login_body_dict = requests_login_body_instance.to_dict()
# create an instance of RequestsLoginBody from a dict
requests_login_body_from_dict = RequestsLoginBody.from_dict(requests_login_body_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


