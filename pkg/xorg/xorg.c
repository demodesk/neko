#include "xorg.h"

static Display *DISPLAY = NULL;

Display *getXDisplay(void) {
  return DISPLAY;
}

int XDisplayOpen(char *name) {
  DISPLAY = XOpenDisplay(name);
  return DISPLAY == NULL;
}

void XDisplayClose(void) {
  XCloseDisplay(DISPLAY);
}

void XMove(int x, int y) {
  Display *display = getXDisplay();
  XWarpPointer(display, None, DefaultRootWindow(display), 0, 0, 0, 0, x, y);
  XSync(display, 0);
}

void XCursorPosition(int *x, int *y) {
  Display *display = getXDisplay();
  Window root = DefaultRootWindow(display);
  Window window;
  int i;
  unsigned mask;
  XQueryPointer(display, root, &root, &window, x, y, &i, &i, &mask);
}

void XScroll(int x, int y) {
  int ydir = 4; /* Button 4 is up, 5 is down. */
  int xdir = 6;

  Display *display = getXDisplay();

  if (y < 0) {
    ydir = 5;
  }

  if (x < 0) {
    xdir = 7;
  }

  int xi;
  int yi;

  for (xi = 0; xi < abs(x); xi++) {
    XTestFakeButtonEvent(display, xdir, 1, CurrentTime);
    XTestFakeButtonEvent(display, xdir, 0, CurrentTime);
  }

  for (yi = 0; yi < abs(y); yi++) {
    XTestFakeButtonEvent(display, ydir, 1, CurrentTime);
    XTestFakeButtonEvent(display, ydir, 0, CurrentTime);
  }

  XSync(display, 0);
}

void XButton(unsigned int button, int down) {
  if (button == 0)
    return;

  Display *display = getXDisplay();
  XTestFakeButtonEvent(display, button, down, CurrentTime);
  XSync(display, 0);
}

static xkeyentry_t *xKeysHead = NULL;

// add keycode->keysym mapping to list
void XKeyEntryAdd(KeySym keysym, KeyCode keycode) {
  xkeyentry_t *entry = (xkeyentry_t *) malloc(sizeof(xkeyentry_t));
  if (entry == NULL)
    return;

  entry->keysym = keysym;
  entry->keycode = keycode;
  entry->next = xKeysHead;
  xKeysHead = entry;
}

// get keycode for keysym from list
KeyCode XKeyEntryGet(KeySym keysym) {
  xkeyentry_t *prev = NULL;
  xkeyentry_t *curr = xKeysHead;

  KeyCode keycode = 0;
  while (curr != NULL) {
    if (curr->keysym == keysym) {
      keycode = curr->keycode;

      if (prev == NULL) {
        xKeysHead = curr->next;
      } else {
        prev->next = curr->next;
      }

      free(curr);
      return keycode;
    }

    prev = curr;
    curr = curr->next;
  }

  return 0;
}

// From https://github.com/TigerVNC/tigervnc/blob/0946e298075f8f7b6d63e552297a787c5f84d27c/unix/x0vncserver/XDesktop.cxx#L343-L379
KeyCode XkbKeysymToKeycode(Display* dpy, KeySym keysym) {
  XkbDescPtr xkb;
  XkbStateRec state;
  unsigned int mods;
  unsigned keycode;

  xkb = XkbGetMap(dpy, XkbAllComponentsMask, XkbUseCoreKbd);
  if (!xkb)
    return 0;

  XkbGetState(dpy, XkbUseCoreKbd, &state);
  // XkbStateFieldFromRec() doesn't work properly because
  // state.lookup_mods isn't properly updated, so we do this manually
  mods = XkbBuildCoreState(XkbStateMods(&state), state.group);

  for (keycode = xkb->min_key_code;
       keycode <= xkb->max_key_code;
       keycode++) {
    KeySym cursym;
    unsigned int out_mods;
    XkbTranslateKeyCode(xkb, keycode, mods, &out_mods, &cursym);
    if (cursym == keysym)
      break;
  }

  if (keycode > xkb->max_key_code)
    keycode = 0;

  XkbFreeKeyboard(xkb, XkbAllComponentsMask, True);

  // Shift+Tab is usually ISO_Left_Tab, but RFB hides this fact. Do
  // another attempt if we failed the initial lookup
  if ((keycode == 0) && (keysym == XK_Tab) && (mods & ShiftMask))
    return XkbKeysymToKeycode(dpy, XK_ISO_Left_Tab);

  return keycode;
}

// From https://github.com/TigerVNC/tigervnc/blob/a434ef3377943e89165ac13c537cd0f28be97f84/unix/x0vncserver/XDesktop.cxx#L401-L453
KeyCode XkbAddKeyKeysym(Display* dpy, KeySym keysym) {
  int types[1];
  unsigned int key;
  XkbDescPtr xkb;
  XkbMapChangesRec changes;
  KeySym *syms;
  KeySym upper, lower;

  xkb = XkbGetMap(dpy, XkbAllComponentsMask, XkbUseCoreKbd);

  if (!xkb)
    return 0;

  for (key = xkb->max_key_code; key >= xkb->min_key_code; key--) {
    if (XkbKeyNumGroups(xkb, key) == 0)
      break;
  }

  // no free keycodes
  if (key < xkb->min_key_code)
    return 0;

  // assign empty structure
  changes = *(XkbMapChangesRec *) malloc(sizeof(XkbMapChangesRec));
  for (int i = 0; i < sizeof(changes); i++) ((char *) &changes)[i] = 0;

  XConvertCase(keysym, &lower, &upper);

  if (upper == lower)
    types[XkbGroup1Index] = XkbOneLevelIndex;
  else
    types[XkbGroup1Index] = XkbAlphabeticIndex;

  XkbChangeTypesOfKey(xkb, key, 1, XkbGroup1Mask, types, &changes);

  syms = XkbKeySymsPtr(xkb,key);
  if (upper == lower)
    syms[0] = keysym;
  else {
    syms[0] = lower;
    syms[1] = upper;
  }

  changes.changed |= XkbKeySymsMask;
  changes.first_key_sym = key;
  changes.num_key_syms = 1;

  if (XkbChangeMap(dpy, xkb, &changes)) {
    return key;
  }

  return 0;
}

void XKey(KeySym keysym, int down) {
  if (keysym == 0)
    return;

  Display *display = getXDisplay();
  KeyCode keycode = 0;

  if (!down)
    keycode = XKeyEntryGet(keysym);

  // Try to get keysyms from existing keycodes
  if (keycode == 0)
    keycode = XkbKeysymToKeycode(display, keysym);

  // Map non-existing keysyms to new keycodes
  if (keycode == 0)
    keycode = XkbAddKeyKeysym(display, keysym);

  if (down)
    XKeyEntryAdd(keysym, keycode);

  XTestFakeKeyEvent(display, keycode, down, CurrentTime);
  XSync(display, 0);
}

Status XSetScreenConfiguration(int width, int height, short *rate) {
  Display *display = getXDisplay();
  Window root = RootWindow(display, 0);
  XRRScreenConfiguration *conf = XRRGetScreenInfo(display, root);

  XRRScreenSize *xrrs;
  int num_sizes;
  xrrs = XRRConfigSizes(conf, &num_sizes);

  int size_index = -1;
  for (int i = 0; i < num_sizes; i++) {
    if (xrrs[i].width == width && xrrs[i].height == height) {
      size_index = i;
      break;
    }
  }

  // if we cannot find the size
  if (size_index == -1) {
    return RRSetConfigFailed;
  }

  short current_rate = 0;
  if (rate != NULL) {
    short *rates;
    int num_rates;
    rates = XRRConfigRates(conf, size_index, &num_rates);

    // try to find the nearest rate
    short nearest_rate = 0;
    float diff = 0;
    for (int i = 0; i < num_rates; i++) {
      if (nearest_rate == 0 || abs(rates[i] - *rate) < diff) {
        nearest_rate = rates[i];
        diff = abs(rates[i] - *rate);
      }
    }

    if (nearest_rate != 0 && diff < 10) {
      current_rate = nearest_rate;
    }

    *rate = current_rate;
  }

  Status status;
  status = XRRSetScreenConfigAndRate(display, conf, root, size_index, RR_Rotate_0, current_rate, CurrentTime);

  XRRFreeScreenConfigInfo(conf);
  return status;
}

void XGetScreenConfiguration(int *width, int *height, short *rate) {
  Display *display = getXDisplay();
  Window root = RootWindow(display, 0);
  XRRScreenConfiguration *conf = XRRGetScreenInfo(display, root);

  Rotation current_rotation;
  SizeID current_size_id = XRRConfigCurrentConfiguration(conf, &current_rotation);

  XRRScreenSize *xrrs;
  int num_sizes;
  xrrs = XRRConfigSizes(conf, &num_sizes);

  // if we cannot find the size
  if (current_size_id >= num_sizes) {
    return;
  }

  *width = xrrs[current_size_id].width;
  *height = xrrs[current_size_id].height;
  *rate = XRRConfigCurrentRate(conf);

  XRRFreeScreenConfigInfo(conf);
}

// Inspired by https://github.com/raboof/xrandr/blob/master/xrandr.c
void XCreateScreenMode(int width, int height, short rate) {
  Display *display = getXDisplay();
  Window root = RootWindow(display, 0);

  char name[128];
  XRRModeInfo mode;
  mode = XCreateScreenModeInfo(width, height, rate);

  snprintf(name, sizeof name, "%dx%d_%d", width, height, rate);
  mode.nameLength = strlen(name);
  mode.name = name;

  // create new mode
  XRRCreateMode(display, root, &mode);
  XSync(display, 0);

  // find newly created mode in resources
  RRMode mode_id;
	XRRScreenResources *resources = XRRGetScreenResources(display, root);
  for (int i = 0; i < resources->nmode; ++i) {
    if (strcmp(resources->modes[i].name, mode.name) == 0) {
      mode_id = resources->modes[i].id;
      break;
    }
  }

  // add new mode to all outputs
  for (int i = 0; i < resources->noutput; ++i) {
    XRRAddOutputMode(display, resources->outputs[i], mode_id);
  }

  XRRFreeScreenResources(resources);
}

// Inspired by https://fossies.org/linux/xwayland/hw/xwayland/xwayland-cvt.c
XRRModeInfo XCreateScreenModeInfo(int hdisplay, int vdisplay, short vrefresh) {
  XRRModeInfo modeinfo;
  memset(&modeinfo, 0, sizeof modeinfo);

#ifdef _LIBCVT_H_
  struct libxcvt_mode_info *mode_info;

  // get screen mode from libxcvt, if available
  mode_info = libxcvt_gen_mode_info(hdisplay, vdisplay, vrefresh, false, false);

  modeinfo.width      = mode_info->hdisplay;
  modeinfo.height     = mode_info->vdisplay;
  modeinfo.dotClock   = mode_info->dot_clock * 1000;
  modeinfo.hSyncStart = mode_info->hsync_start;
  modeinfo.hSyncEnd   = mode_info->hsync_end;
  modeinfo.hTotal     = mode_info->htotal;
  modeinfo.vSyncStart = mode_info->vsync_start;
  modeinfo.vSyncEnd   = mode_info->vsync_end;
  modeinfo.vTotal     = mode_info->vtotal;
  modeinfo.modeFlags  = mode_info->mode_flags;

  free(mode_info);
#else
  // fallback to a simple mode without refresh rate
  modeinfo.width = hdisplay;
  modeinfo.height = vdisplay;
#endif

  return modeinfo;
}

void XSetKeyboardModifier(int mod, int on) {
  Display *display = getXDisplay();
  XkbLockModifiers(display, XkbUseCoreKbd, mod, on ? mod : 0);
  XFlush(display);
}

char XGetKeyboardModifiers() {
  XkbStateRec xkbState;
  Display *display = getXDisplay();
  XkbGetState(display, XkbUseCoreKbd, &xkbState);
  return xkbState.locked_mods;
}

XFixesCursorImage *XGetCursorImage(void) {
  Display *display = getXDisplay();
  return XFixesGetCursorImage(display);
}

char *XGetScreenshot(int *w, int *h) {
  Display *display = getXDisplay();
  Window root = DefaultRootWindow(display);

  XWindowAttributes attr;
  XGetWindowAttributes(display, root, &attr);
  int width = attr.width;
  int height = attr.height;

  XImage *ximage = XGetImage(display, root, 0, 0, width, height, AllPlanes, ZPixmap);

  *w = width;
  *h = height;
  char *pixels = (char *)malloc(width * height * 3);

  for (int row = 0; row < height; row++) {
    for (int col = 0; col < width; col++) {
      int pos = ((row * width) + col) * 3;
      unsigned long pixel = XGetPixel(ximage, col, row);

      pixels[pos]   = (pixel & ximage->red_mask)   >> 16;
      pixels[pos+1] = (pixel & ximage->green_mask) >> 8;
      pixels[pos+2] =  pixel & ximage->blue_mask;
    }
  }

  XDestroyImage(ximage);
  return pixels;
}
