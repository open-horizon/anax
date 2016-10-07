/* The JavaScript that is used in iotf.html for device iotf registration */
// check internet connectivity
$.ajax({
    url: '/info',
    type: 'GET',
    crossDomain: true,
    dataType: 'json',
    success: function(data) {
        var bad_servers = [];
        for (var key in data.connectivity) {
            if (!data.connectivity[key]) {
                bad_servers.push(key);
            }
        }

        if (bad_servers.length == 0) {
            disp_app_section(0, 'precheck');
        } else {
            document.getElementById('precheck_warn').innerHTML = "The ip address for the following servers cannot be resolved:\n&nbsp;&nbsp;&nbsp;&nbsp;" + bad_servers.join();
            disp_app_section(1, 'precheck');
        }
        var str = JSON.stringify(data.connectivity, null, 2);
        console.log("Checking internet connectivity: " + str);
    },
    error: function(xhr, status, err) {
        document.getElementById('precheck_warn').innerHTML = ""
        disp_app_section(1, 'precheck');
        console.log("Error checking internet connectivity: " + status + err);
    },
});

var app_selections = ["Edge Analytics Agent"]

// populate the selection box
var selection_box = document.getElementById('iotf_app');
for (var i = 0; i < app_selections.length; i++) {
    var item = document.createElement('option');
    item.innerHTML = app_selections[i];
    item.value = app_selections[i];
    selection_box.appendChild(item);
}

// default selection
selection_box.value = "Edge Analytics Agent";

$('#submit').click(function() {
    var iotf_app = document.getElementById('iotf_app').value;
    var iotf_org = document.getElementById('iotf_org').value;
    var iotf_gatewayid = document.getElementById('iotf_gatewayid').value;
    var iotf_gatewaytype = document.getElementById('iotf_gatewaytype').value;
    var iotf_authmethod = document.getElementById('iotf_authmethod').value;
    var iotf_authtoken = document.getElementById('iotf_authtoken').value;
    var payload = {
        'ram': 512,
        'hourly_cost_bacon': 480,
        'lat': '0.0',
        'lon': '0.0',
        'name': 'IoTF Contract',
        'app_attributes': {
            'iotf': "true",
            'iotf_app': iotf_app,
            'iotf_org': iotf_org,
            'iotf_gatewayid': iotf_gatewayid,
            'iotf_gatewaytype': iotf_gatewaytype,
            'iotf_authmethod': iotf_authmethod,
            'iotf_authtoken': iotf_authtoken,
        },
    };

    $.ajax({
        url: '/contract',
        type: 'POST',
        data: JSON.stringify(payload),
        crossDomain: true,
        contentType: 'application/json',
        success: function(data) {
            window.location = '/registration/complete.html';
        },
        error: function(xhr, status, err) {
            console.log("Error POSTing requests", status, err);
            if (err == "status code 1000") {
                alert("The registration cannot procceed because the device does not have internet connection.");

            } else {
                alert("The registration cannot procceed because the server returns " + err + ".");
            }
        }
    });
});
