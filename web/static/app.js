(() => {
  const meta = document.querySelector('meta[name="csrf-token"]');
  const CSRF = meta ? meta.content : "";

  // Expose a helper for AJAX wishlist adds
  window.addToWishlist = async function(productId){
    const fd = new FormData();
    fd.append("product_id", productId);
    fd.append("csrf", CSRF);  // form-field mode matches main.go KeyLookup
    const res = await fetch("/wishlist", {
      method: "POST",
      credentials: "same-origin",
      body: fd
    });
    if (!res.ok) {
      alert("Wishlist failed (maybe CSRF/403).");
      return;
    }
    // optional: refresh list / update UI
    location.reload();
  };
})();
