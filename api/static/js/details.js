/* The JavaScript that is used in details.html for device registration */
var ACCOUNT = null;

// get the name of the contract that will be displayed
var Contract_Name = get_url_param(window.location.href, "name");
Contract_Name = Contract_Name.replace(/%20/g, ' ')

// get the data from the server to display
fetchInfo(Contract_Name);

// convert the timestamp to date
function toDate(seconds) {
    if (seconds == 0) {
        return 0;
    }

    var d = new Date(0);
    d.setUTCSeconds(seconds);
    return d;
};

// get the contract info
function fetchInfo(con_name) {
    if (ACCOUNT === null) {
        $.ajax({
            url: '/status',
            type: 'GET',
            dataType: 'json',
            success: function(data) {
                disp_accounts(data);
            },
            error: function(xhr, status, err) {
                console.log("Error getting contract info: " + err + ". Return code is " + status);
            }
        });
    }

    $.ajax({
        url: '/agreement/all',
        type: 'GET',
        dataType: 'json',
        success: function(data) {
            disp_contract_details(data, con_name);
        },
        error: function(xhr, status, err) {
            disp_single_row("Error getting contract info: " + err + ". Return code is " + status);
        }
    });

}


// add a row in the table with given values.
function add_rows(con) {
    var table = document.getElementById("contract_details");
    for (var key in con) {
        var newrow = table.insertRow(-1);
        var attr_cell = newrow.insertCell(0);
        var val_cell = newrow.insertCell(1);
        attr_cell.innerHTML = key;
        val_cell.innerHTML = con[key];;
    }
}

function disp_single_row(text) {
    //remove all the rows except the title
    $("#contract_details").find("tr:not(:first)").remove();

    // display a single row with text
    var table = document.getElementById("contract_details");
    var row = table.insertRow(-1);
    var cell = row.insertCell(0);
    cell.colSpan = 3
    cell.innerHTML = text;
}


// dispay the contract attributes and values on the table
function disp_contract_details(data, con_name) {
    $.each($.map(data.active, function(con) {
        var services = $.map(con.current_deployment, function(n, key) {
            return key;
        });

        if (con.name == con_name) {
            // display it on the table
            var it = {
                "name": con.name,
                "contract address": con.contract_address,
                "agreement id": con.current_agreement_id,
                "agreement proposal received": toDate(con.agreement_creation_time),
                "code downloaded and verified": toDate(con.agreement_execution_start_time),
                "in contract ": toDate(con.agreement_accepted_time),
                "workloads currently running on the device": services
            };
            add_rows(it);

            // diplay env variables
            if ("environment_additions" in con) {
                var env_data = {}
                for (var key in con.environment_additions) {
                    switch (key) {
                        case "MTN_NAME":
                        case "MTN_AGREEMENTID":
                        case "MTN_CONFIGURE_NONCE":
                        case "MTN_CONTRACT":
                            break;
                        case "MTN_LAT":
                            env_data['latitude'] = con.environment_additions[key];
                            break;
                        case "MTN_LON":
                            env_data['longitude'] = con.environment_additions[key];
                            break;
                        case "MTN_IS_LOC_ENABLED":
                            env_data['share gps coordinates with workloads'] = con.environment_additions[key];
                            break;
                        default:
                            var newkey = key.replace('MTN_', '');
                            newkey = newkey.replace(/_/g, ' ');
                            env_data[newkey.toLowerCase()] = con.environment_additions[key];
                    }
                }
                add_rows(env_data);
            }
        }

    }));
}

// dispay ethereum account if it is not null
function disp_accounts(data) {
    if (data !== null) {
        if (data.geth !== null) {
            if (data.geth.eth_accounts !== null) {
                ACCOUNT = data.geth.eth_accounts.join(',');
                document.getElementById("account").innerHTML = "Account: " + ACCOUNT;
            }
        }
    }
}


//go back to status.html when back button is clicked
$('#back').click(function() {
    window.history.back();
});
