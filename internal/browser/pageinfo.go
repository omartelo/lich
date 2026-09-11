package browser

// pageInfoScript is injected into the page the agent is driving. It stamps
// data-lich-idx so a later click/type can find the same node, and it never
// reads a form control's value into the result.
//
// A password input has no innerText, so an innerText || value label chain
// hands the live credential to the agent and into the session transcript.
// Rather than classify which fields are sensitive — type === 'password' is
// defeated by a show-password toggle, and an ordinary text input can still
// hold a recovery phrase — no control value is serialized at all. Agents get
// filled / checked instead, which is enough to confirm that typing landed
// without the content leaving the page.
//
// A textarea is covered by the same rule: its content is its default value, so
// innerText on a textarea is a value read. The single exception is a
// push-button input (submit / button / reset), whose value is the author-written
// caption and the only label it has. A user cannot type into one.
const pageInfoScript = `(() => {
  const sel = 'a[href], button, input, textarea, select, [role=button], [role=link], [onclick], [contenteditable=""], [contenteditable="true"]';
  const isVisible = (el) => {
    const r = el.getBoundingClientRect();
    if (r.width <= 0 || r.height <= 0) return false;
    const s = getComputedStyle(el);
    return s.visibility !== 'hidden' && s.display !== 'none';
  };
  const CAPTION_TYPES = ['submit', 'button', 'reset'];
  const attr = (el, name) => el.getAttribute(name) || '';
  const isCaptionInput = (tag, type) =>
    tag === 'input' && CAPTION_TYPES.indexOf(type) !== -1;
  const labelFor = (el, tag, type) => {
    if (tag === 'select') {
      const opt = el.options ? el.options[el.selectedIndex] : null;
      const optText = opt ? (opt.textContent || '').trim() : '';
      if (optText) return optText;
    } else if (isCaptionInput(tag, type)) {
      const caption = (el.value || '').trim();
      if (caption) return caption;
    } else if (tag !== 'input' && tag !== 'textarea') {
      const own = (el.innerText || '').trim();
      if (own) return own;
    }
    const labelled = el.labels && el.labels[0];
    const labelText = labelled ? (labelled.innerText || labelled.textContent || '').trim() : '';
    if (labelText) return labelText;
    return (
      attr(el, 'aria-label') ||
      attr(el, 'title') ||
      attr(el, 'placeholder') ||
      attr(el, 'name')
    ).trim();
  };
  const els = [...document.querySelectorAll(sel)].filter(isVisible);
  const interactive = els.slice(0, 200).map((el, i) => {
    el.setAttribute('data-lich-idx', String(i));
    const r = el.getBoundingClientRect();
    const tag = el.tagName.toLowerCase();
    const type = (el.getAttribute('type') || '').toLowerCase();
    const entry = {
      index: i,
      tag: tag,
      type: el.getAttribute('type') || undefined,
      role: el.getAttribute('role') || undefined,
      text: labelFor(el, tag, type).slice(0, 120),
      href: el.getAttribute('href') || undefined,
      rect: { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) },
    };
    if (type === 'checkbox' || type === 'radio') {
      entry.checked = el.checked === true;
    } else if (tag === 'textarea' || (tag === 'input' && !isCaptionInput(tag, type))) {
      entry.filled = typeof el.value === 'string' && el.value.length > 0;
    }
    return entry;
  });
  return {
    url: location.href,
    title: document.title,
    text: (document.body ? document.body.innerText : '').trim().slice(0, 5000),
    interactive: interactive,
    truncated: els.length > 200,
  };
})()`
