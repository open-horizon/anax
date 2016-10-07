/* The JavaScript functions that are used by other scripts for device registration */
//check if a string is a positive integer
function isPositiveInteger(value) {
    return (/^[\d]+$/).test(value.trim());
}

// change the input backgroud red and show the error text if there is error
function change_display(has_error, input_field_name, error_field_name) {
    if (has_error) {
        //document.getElementById(input_field_name).style.background = 'red';
        document.getElementById(input_field_name).style.borderColor = 'red';
        document.getElementById(error_field_name).style.display = "block";
    } else {
        // restore the intput background and hide the error text
        document.getElementById(input_field_name).style.borderColor = '';
        document.getElementById(error_field_name).style.display = "none";
    }
}

//check if a string is a number
function isNumber(value) {
    return (/^[-]?[\d]+$/).test(value.trim());
}

//check if a string is a real
function isReal(value) {
    return (/^[-]*[\d]+[\.]?[\d]*$/).test(value.trim());
}

// turning the applications on/off according to the user configurations
function disp_app_section(show, id) {
    if (show) {
        document.getElementById(id).style.display = "block";
    } else {
        document.getElementById(id).style.display = "none";
    }
}

function get_url_param(url_in, param) {
    var results = new RegExp('[\?&]' + param + '=([^&#]*)').exec(url_in);
    return results[1] || 0;
}
