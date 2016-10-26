/* The JavaScript that is used in status.html for device registration */
var name_column_index = 0;
var status_column_index = 1;

// ethereum account
var ACCOUNT = null;

// map agreement id to contract name
var Agreement_map = {};

// contract status
var STATUS = ["Waiting to Advertise", "Advertised", "Negotiating", "Downloading Workload", "Running Workload", "In Contract"];

// set up the refresh interval for the table.  
$(document).ready(function() {
    fetch_info();
    // refresh table after 30 secs
    setInterval(function() {
        fetch_info();
    }, 15000);
});


// get the contract info from the server.
function fetch_info() {
    if (ACCOUNT === null) {
        $.ajax({
            url: '/info',
            type: 'GET',
            dataType: 'json',
            success: function(data) {
                disp_accounts(data);
            },
            error: function(xhr, status, err) {
                console.log("Error getting account: " + err + ". Return code is " + status);
            }
        });
    }

    $.ajax({
        url: '/contract',
        type: 'GET',
        dataType: 'json',
        success: function(data) {
            // hide the error row if any
            var row = document.getElementById('single_row');
            if (row !== null) {
                row.style.display = 'none';
            }

            // now display the data
            disp_contracts(data);
        },
        error: function(xhr, status, err) {
            disp_single_row("Error getting contract info: " + err + ". Return code is " + status);
        }
    });
}


// create a status string for the given contract
function get_contract_status(contract) {
    var status = STATUS[1];
    // if the status has already defined in the contract, just use it.
    if ("status" in contract && contract.status !== null && contract.status != "") {
        return status;
    }

    if (contract.agreement_terminated_time != 0) {
        status = "Ended"
    } else {
        if (contract.agreement_finalized_time != 0) {
            status = STATUS[5];
        } else if (contract.agreement_execution_start_time != 0) {
            status = STATUS[4];
        } else if (contract.agreement_accepted_time != 0) {
            status = STATUS[3];
        }  else if (contract.agreement_creation_time != 0) {
            status = STATUS[2];
        } else {
            status = STATUS[1]; 
        }
    }

    return status;
}


// update table with the given contract. 
// If the contract is in the table, update it. Otherwise, create a new row
// to display it.
function update_contract_table(contract) {
    var status = get_contract_status(contract);
    var table = document.getElementById("contract_status");
    var found = false;
    var name_cell, status_cell;
    for (var i = 0, row; row = table.rows[i]; i++) {
        if (row.cells[name_column_index].innerText.startsWith(contract.name)) {
            name_cell = row.cells[name_column_index];
            status_cell = row.cells[status_column_index];
            found = true;
            break;
        }
    }

    // if the contract is not in the table yet, create a new row
    if (!found) {
        var newrow = table.insertRow(-1);
        name_cell = newrow.insertCell(name_column_index);
        status_cell = newrow.insertCell(status_column_index);
    }

    if (status == STATUS[0]) {
        // contract is not yet registered, it did not come from the server
        name_cell.innerHTML = contract.name;
        name_cell.style = "white-space:nowrap";
    } else {
        // the contract info is from the server, so we have a link to show the details
        name_cell.innerHTML = "<u>" + contract.name.link("/registration/details.html?name=" + contract.name) + "</u><br><small>" + contract.agreement_id + "</small>";
        name_cell.style = "white-space:pre";
    }

    // for status other than "in contract", need a status bar if it is not there.
    var found_bar = false;
    var children = status_cell.childNodes;
    for (var i = 0; i < children.length; i++) {
        if (children[i].className == 'statusProgress') {
            found_bar = true;
        }
    }
    if (!found_bar) {
        if (status !== STATUS[5]) {
            status_cell.style.width = "99%";
            status_cell.innerHTML = '<div class="statusProgress"> <div class="statusBar"></div></div><label class="statusLabel">' + status + '</label>';
        } else {
            status_cell.innerHTML = status;
        }
    }

    // get index of the current status
    var index = STATUS.findIndex(function(s) {
        return s == status;
    });

    //update the status bar and the label
    var children = status_cell.childNodes;
    for (var i = 0; i < children.length; i++) {
        if (children[i].className == 'statusProgress') {
            var grandchildren = children[0].childNodes;
            for (var j = 0; j < grandchildren.length; j++) {
                if (grandchildren[j].className == 'statusBar') {
                    var percent = Math.floor((index / (STATUS.length - 1)) * 100);
                    grandchildren[j].style.width = percent.toString() + '%';
                    break;
                }
            }
        } else if (children[i].className == 'statusLabel') {
             children[i].innerHTML = status;
        }
    }
}


// display the given message in the table in a single row.
// it is used to display error messages.
function disp_single_row(text) {
    //remove all the rows except the title
    $("#contract_status").find("tr:not(:first)").remove();

    // display a single row with text
    var table = document.getElementById("contract_status");
    var row = table.insertRow(-1);
    row.id = 'single_row';
    var cell = row.insertCell(0);
    cell.colSpan = 3
    cell.style = "text-align: center;"
    cell.innerHTML = text;
}


// display the contracts on the table. it includes the established contract
// as well as the pending contract.
function disp_contracts(data) {
    var total = 0;
    var pending_cons = [];
    var contract_submitted = sessionStorage.getItem("contract_submitted");
    if (contract_submitted !== null && contract_submitted != "") {
        pending_cons = contract_submitted.split(',');
    }

    $.each($.map(data.contracts, function(con) {
        var services = $.map(con.current_deployment, function(n, key) {
            return key;
        });

        // display it on the table
        var it = {
            "name": con.name,
            "agreement_id": con.current_agreement_id,
            "agreement_creation_time": con.agreement_creation_time,
            "agreement_accepted_time": con.agreement_accepted_time,
            "agreement_finalized_time":con.agreement_finalized_time,
            "agreement_execution_start_time": con.agreement_execution_start_time,
            "agreement_terminated_time":con.agreement_terminated_time,
            "services": services,
        };
        update_contract_table(it);
        total++;

        // add it the agreement map
        if (it.agreement !== null && it.agreement !== "") {
            Agreement_map[it.agreement] = it.name;
        }

        // remove it from the pending contract list
        if (pending_cons.length > 0) {
            var index = pending_cons.indexOf(con.name);
            if (index > -1) {
                pending_cons.splice(index, 1);
                sessionStorage.setItem("contract_submitted", pending_cons.join(','));
            }
        }
    }));

    // now display the pending contract
    if (pending_cons.length > 0) {
        pending_cons.forEach(function(con) {
            var it = {
                "name": con,
                "agreement_id": "0",
                "agreement_creation_time": 0,
                "agreement_accepted_time": 0,
                "agreement_finalized_time": 0,
                "agreement_execution_start_time": 0,
                "agreement_terminated_time": 0,
                "status": STATUS[0],
            };
            update_contract_table(it);
            total++;

        });
    }

    // display a single row to let the user know that there is no data to display.
    if (total == 0) {
        disp_single_row("No data available.")
    }
}

// display ethereum account if it is not null
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
