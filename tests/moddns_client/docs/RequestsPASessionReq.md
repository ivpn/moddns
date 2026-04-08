# RequestsPASessionReq


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **str** |  | 
**preauth_id** | **str** |  | 
**token** | **str** |  | 

## Example

```python
from moddns.models.requests_pa_session_req import RequestsPASessionReq

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsPASessionReq from a JSON string
requests_pa_session_req_instance = RequestsPASessionReq.from_json(json)
# print the JSON string representation of the object
print(RequestsPASessionReq.to_json())

# convert the object into a dict
requests_pa_session_req_dict = requests_pa_session_req_instance.to_dict()
# create an instance of RequestsPASessionReq from a dict
requests_pa_session_req_from_dict = RequestsPASessionReq.from_dict(requests_pa_session_req_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


