# RequestsRotatePASessionReq


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**sessionid** | **str** |  | 

## Example

```python
from moddns.models.requests_rotate_pa_session_req import RequestsRotatePASessionReq

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsRotatePASessionReq from a JSON string
requests_rotate_pa_session_req_instance = RequestsRotatePASessionReq.from_json(json)
# print the JSON string representation of the object
print(RequestsRotatePASessionReq.to_json())

# convert the object into a dict
requests_rotate_pa_session_req_dict = requests_rotate_pa_session_req_instance.to_dict()
# create an instance of RequestsRotatePASessionReq from a dict
requests_rotate_pa_session_req_from_dict = RequestsRotatePASessionReq.from_dict(requests_rotate_pa_session_req_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


