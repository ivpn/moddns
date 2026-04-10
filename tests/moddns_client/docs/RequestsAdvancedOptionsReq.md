# RequestsAdvancedOptionsReq


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**encryption_type** | **str** |  | 
**excluded_wifi_networks** | **str** | ExcludedDomains          string &#x60;json:\&quot;excluded_domains\&quot;&#x60; | [optional] 

## Example

```python
from moddns.models.requests_advanced_options_req import RequestsAdvancedOptionsReq

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsAdvancedOptionsReq from a JSON string
requests_advanced_options_req_instance = RequestsAdvancedOptionsReq.from_json(json)
# print the JSON string representation of the object
print(RequestsAdvancedOptionsReq.to_json())

# convert the object into a dict
requests_advanced_options_req_dict = requests_advanced_options_req_instance.to_dict()
# create an instance of RequestsAdvancedOptionsReq from a dict
requests_advanced_options_req_from_dict = RequestsAdvancedOptionsReq.from_dict(requests_advanced_options_req_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


