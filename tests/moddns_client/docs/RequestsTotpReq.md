# RequestsTotpReq


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**otp** | **str** |  | 

## Example

```python
from moddns.models.requests_totp_req import RequestsTotpReq

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsTotpReq from a JSON string
requests_totp_req_instance = RequestsTotpReq.from_json(json)
# print the JSON string representation of the object
print(RequestsTotpReq.to_json())

# convert the object into a dict
requests_totp_req_dict = requests_totp_req_instance.to_dict()
# create an instance of RequestsTotpReq from a dict
requests_totp_req_from_dict = RequestsTotpReq.from_dict(requests_totp_req_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


