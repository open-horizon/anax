{% assign code = include.code %}
{% assign language = include.language %}

``` {{ language }}
{{ code }}
```
{% assign nanosecond = "now" | date: "%N" %}
<button id="copybutton{{ nanosecond }}" data-clipboard-text="{{ code | xml_escape }}" aria-label="Copied!">
  Copy to clipboard
</button>

<script>
var clipboard{{ nanosecond }} = new ClipboardJS('#copybutton{{ nanosecond }}');

clipboard{{ nanosecond }}.on('success', function(e) {

    // Set Button HTML to 'Copied!'
    document.querySelector('#copybutton{{ nanosecond }}').innerHTML = 'Copied!';

    // After 2 seconds, return back to original text.
    setTimeout(() => {
     document.querySelector('#copybutton{{ nanosecond }}').innerHTML = 'Copy to clipboard';
    }, 2000);

    /* Testing custom logic through console
    console.info('Action:', e.action);
    console.info('Text:', e.text);
    console.info('Trigger:', e.trigger);
    */

    console.log(e);
    
});
clipboard{{ nanosecond }}.on('error', function(e) {

    /* Testing custom logic through console
    console.info('Action:', e.action);
    console.info('Trigger:', e.trigger);
    */

    console.log(e);
});
</script>