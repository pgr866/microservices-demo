// Not inline in the template, so the Content-Security-Policy can forbid
// inline scripts.
document.addEventListener("DOMContentLoaded", function () {
  var form = document.getElementById("currency_form");
  if (form) {
    form.elements.currency_code.addEventListener("change", function () {
      form.submit();
    });
  }
});
