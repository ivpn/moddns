# RequestsMobileConfigReq


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**advanced_options** | [**RequestsAdvancedOptionsReq**](RequestsAdvancedOptionsReq.md) |  | [optional] 
**device_id** | **str** | DeviceId is an optional human-friendly identifier for the device. It will be normalized (allowing only [A-Za-z0-9 -]) and truncated to a max length consistent with the DNS proxy rules (currently 16). When provided, generated mobileconfig profile endpoints (DoH / DoT / DoQ) will embed it so queries can be attributed per-device. | [optional] 
**profile_id** | **str** |  | 

## Example

```python
from moddns.models.requests_mobile_config_req import RequestsMobileConfigReq

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsMobileConfigReq from a JSON string
requests_mobile_config_req_instance = RequestsMobileConfigReq.from_json(json)
# print the JSON string representation of the object
print(RequestsMobileConfigReq.to_json())

# convert the object into a dict
requests_mobile_config_req_dict = requests_mobile_config_req_instance.to_dict()
# create an instance of RequestsMobileConfigReq from a dict
requests_mobile_config_req_from_dict = RequestsMobileConfigReq.from_dict(requests_mobile_config_req_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


