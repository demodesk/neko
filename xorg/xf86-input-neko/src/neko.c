/*
 * (c) 2017 Martin Kepplinger <martink@posteo.de>
 * (c) 2007 Clement Chauplannaz, Thales e-Transactions <chauplac@gmail.com>
 * (c) 2006 Sascha Hauer, Pengutronix <s.hauer@pengutronix.de>
 *
 * derived from the xf86-input-void driver
 * Copyright 1999 by Frederic Lepied, France. <Lepied@XFree86.org>
 *
 * Permission to use, copy, modify, distribute, and sell this software and its
 * documentation for any purpose is  hereby granted without fee, provided that
 * the  above copyright   notice appear  in   all  copies and  that both  that
 * copyright  notice   and   this  permission   notice  appear  in  supporting
 * documentation, and that   the  name of  Frederic   Lepied not  be  used  in
 * advertising or publicity pertaining to distribution of the software without
 * specific,  written      prior  permission.     Frederic  Lepied   makes  no
 * representations about the suitability of this software for any purpose.  It
 * is provided "as is" without express or implied warranty.
 *
 * FREDERIC  LEPIED DISCLAIMS ALL   WARRANTIES WITH REGARD  TO  THIS SOFTWARE,
 * INCLUDING ALL IMPLIED   WARRANTIES OF MERCHANTABILITY  AND   FITNESS, IN NO
 * EVENT  SHALL FREDERIC  LEPIED BE   LIABLE   FOR ANY  SPECIAL, INDIRECT   OR
 * CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE,
 * DATA  OR PROFITS, WHETHER  IN  AN ACTION OF  CONTRACT,  NEGLIGENCE OR OTHER
 * TORTIOUS  ACTION, ARISING    OUT OF OR   IN  CONNECTION  WITH THE USE    OR
 * PERFORMANCE OF THIS SOFTWARE.
 *
 * SPDX-License-Identifier: MIT
 * License-Filename: COPYING
 */

/* neko input driver */
// https://www.x.org/releases/X11R7.7/doc/xorg-server/Xinput.html

#ifdef HAVE_CONFIG_H
#include "config.h"
#endif

#define SOCKET_NAME "/tmp/resol.sock"
#define BUFFER_SIZE 12

#include <stdio.h>
#include <stdio.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <misc.h>
#include <xf86.h>
#if !defined(DGUX)
#include <xisb.h>
#endif
#include <xf86_OSproc.h>
#include <xf86Xinput.h>
#include <exevents.h> /* Needed for InitValuator/Proximity stuff */
#include <X11/keysym.h>
#include <mipointer.h>
#include <xserver-properties.h>
#include <pthread.h>

#define MAXBUTTONS 11   /* > 10 */
#define TOUCH_NUM_AXES 3 /* x, y, pressure */
#define TOUCH_MAX_SLOTS 10

struct neko_sample
{
    int type;
    int touchId;
    int x;
    int y;
    unsigned int pressure;
};

struct neko_priv
{
    pthread_t thread;
    int height;
    int width;
    int pmax;
    ValuatorMask *valuators;
    int8_t abs_x_only;
    uint16_t slots;

    struct sockaddr_un addr;
    int listen_socket;
};

// from binary representation to struct
static void unpackNekoSample(struct neko_sample *samp, unsigned char *buffer)
{
    samp->type = buffer[0];
    samp->touchId = buffer[1];
    samp->x = buffer[2] << 8 | buffer[3];
    samp->y = buffer[4] << 8 | buffer[5];
    samp->pressure = buffer[6] << 8 | buffer[7];
}

static void xf86NekoReadInput(InputInfoPtr local)
{
    struct neko_priv *priv = (struct neko_priv *) (local->private);
    struct neko_sample samp;
    int ret;

    int data_socket;
    unsigned char buffer[BUFFER_SIZE];

    for (;;)
    {
        /* Wait for incoming connection. */

        data_socket = accept(priv->listen_socket, NULL, NULL);
        if (data_socket == -1)
        {
            perror("accept");
            exit(EXIT_FAILURE);
        }

        fprintf(stderr, "xf86-input-neko: accepted\n");

        for(;;)
        {
            /* Wait for next data packet. */

            ret = read(data_socket, buffer, BUFFER_SIZE);
            if (ret == -1)
            {
                perror("read");
                exit(EXIT_FAILURE);
            }

            if (ret == 0)
            {
                fprintf(stderr, "xf86-input-neko: read 0 bytes\n");
                break;
            }

            fprintf(stderr, "xf86-input-neko: read %d bytes\n", ret);

            unpackNekoSample(&samp, buffer);

            ValuatorMask *m = priv->valuators;
            valuator_mask_zero(m);
            valuator_mask_set_double(m, 0, samp.x);
            valuator_mask_set_double(m, 1, samp.y);
            valuator_mask_set_double(m, 2, samp.pressure);

            xf86PostTouchEvent(local->dev, samp.touchId, samp.type, 0, m);
            fprintf(stderr, "xf86-input-neko: touchId is %d, type is %d\n", samp.touchId, samp.type);
            fprintf(stderr, "xf86-input-neko: x is %d, y is %d, pressure is %d\n", samp.x, samp.y, samp.pressure);
        }

        /* Close socket. */
        close(data_socket);
    }
}

static int xf86NekoControlProc(DeviceIntPtr device, int what)
{
    // device pInfo
    InputInfoPtr pInfo = device->public.devicePrivate;
    // custom private data
    struct neko_priv *priv = pInfo->private;

    switch (what) {
    case DEVICE_INIT:
        device->public.on = FALSE;

        unsigned char map[MAXBUTTONS + 1];
        Atom labels[MAXBUTTONS];
        Atom axis_labels[TOUCH_NUM_AXES];

        // init button map
        memset(map, 0, sizeof(map));
        for (int i = 0; i < MAXBUTTONS; i++)
        {
            map[i + 1] = i + 1;
        }

        // init labels
        memset(labels, 0, ARRAY_SIZE(labels) * sizeof(Atom));
        labels[0] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_LEFT);
        labels[1] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_MIDDLE);
        labels[2] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_RIGHT);
        labels[3] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_WHEEL_UP);
        labels[4] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_WHEEL_DOWN);
        labels[5] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_HWHEEL_LEFT);
        labels[6] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_HWHEEL_RIGHT);
        labels[7] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_SIDE);
        labels[8] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_EXTRA);
        labels[9] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_FORWARD);
        labels[10] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_BACK);

        // init axis labels
        memset(axis_labels, 0, ARRAY_SIZE(axis_labels) * sizeof(Atom));
        if (priv->abs_x_only)
        {
            axis_labels[0] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_X);
            axis_labels[1] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_Y);
            axis_labels[2] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_PRESSURE);
        }
        else
        {
            axis_labels[0] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_X);
            axis_labels[1] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_Y);
            axis_labels[2] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_PRESSURE);
        }

        /*
            This function is provided to allocate and initialize a ButtonClassRec
            and should be called for extension devices that have buttons.
            It is passed a pointer to the device, the number of buttons supported,
            and a map of the reported button codes.
            It returns FALSE if the ButtonClassRec could not be allocated.

            Bool InitButtonClassDeviceStruct(dev, numButtons, map)
                    register DeviceIntPtr dev;
                    int numButtons;
                    CARD8 *map;
        */
        if (InitButtonClassDeviceStruct(device,
                MAXBUTTONS,
                labels,
                map) == FALSE)
        {
            xf86IDrvMsg(pInfo, X_ERROR,
                "unable to allocate Button class device\n");
            return !Success;
        }

        /* 
            This function is provided to allocate and initialize a ValuatorClassRec,
            and should be called for extension devices that have valuators. It is
            passed the number of axes of motion reported by the device, the address
            of the motion history procedure for the device, the size of the motion
            history buffer, and the mode (Absolute or Relative) of the device.
            It returns FALSE if the ValuatorClassRec could not be allocated.

            Bool InitValuatorClassDeviceStruct(dev, numAxes, motionProc, numMotionEvents, mode)
                DeviceIntPtr dev;
                int (*motionProc)();
                int numAxes;
                int numMotionEvents;
                int mode;
        */
        if (InitValuatorClassDeviceStruct(device,
                TOUCH_NUM_AXES,
                axis_labels,
                0, Absolute) == FALSE)
        {
            xf86IDrvMsg(pInfo, X_ERROR,
                "unable to allocate Valuator class device\n");
            return !Success;
        }

        /* 
            This function is provided to initialize an XAxisInfoRec, and should be
            called for core and extension devices that have valuators. The space
            for the XAxisInfoRec is allocated by the InitValuatorClassDeviceStruct
            function, but is not initialized.
            
            InitValuatorAxisStruct should be called once for each axis of motion
            reported by the device. Each invocation should be passed the axis
            number (starting with 0), the minimum value for that axis, the maximum
            value for that axis, and the resolution of the device in counts per meter.
            If the device reports relative motion, 0 should be reported as the
            minimum and maximum values.

            InitValuatorAxisStruct(dev, axnum, minval, maxval, resolution)
                DeviceIntPtr dev;
                int axnum;
                int minval;
                int maxval;
                int resolution;
        */
        if (priv->abs_x_only) {
            InitValuatorAxisStruct(device, 0,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_X),
                0,                /* min val */
                priv->width - 1,  /* max val */
                priv->width,      /* resolution */
                0,                /* min_res */
                priv->width,      /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 1,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_Y),
                0,                /* min val */
                priv->height - 1, /* max val */
                priv->height,     /* resolution */
                0,                /* min_res */
                priv->height,     /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 2,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_PRESSURE),
                0,                /* min val */
                priv->pmax,       /* max val */
                priv->pmax + 1,   /* resolution */
                0,                /* min_res */
                priv->pmax + 1,   /* max_res */
                Absolute);
        }
        else
        {
            InitValuatorAxisStruct(device, 0,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_X),
                0,                /* min val */
                priv->width - 1,  /* max val */
                priv->width,      /* resolution */
                0,                /* min_res */
                priv->width,      /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 1,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_Y),
                0,                /* min val */
                priv->height - 1, /* max val */
                priv->height,     /* resolution */
                0,                /* min_res */
                priv->height,     /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 2,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_PRESSURE),
                0,                /* min val */
                priv->pmax,       /* max val */
                priv->pmax + 1,   /* resolution */
                0,                /* min_res */
                priv->pmax + 1,   /* max_res */
                Absolute);
        }

        /*
            The mode field is either XIDirectTouch for direct−input touch devices
            such as touchscreens or XIDependentTouch for indirect input devices such
            as touchpads. For XIDirectTouch devices, touch events are sent to window
            at the position the touch occured. For XIDependentTouch devices, touch
            events are sent to the window at the position of the device's sprite.

            The num_touches field defines the maximum number of simultaneous touches
            the device supports. A num_touches of 0 means the maximum number of
            simultaneous touches is undefined or unspecified. This field should be
            used as a guide only, devices will lie about their capabilities.
        */
        if (InitTouchClassDeviceStruct(device,
                priv->slots,
                XIDirectTouch,
                TOUCH_NUM_AXES) == FALSE)
        {
            xf86IDrvMsg(pInfo, X_ERROR,
                "Unable to allocate TouchClassDeviceStruct\n");
            return !Success;
        }

        break;

    case DEVICE_ON:
        xf86IDrvMsg(pInfo, X_PROBED, "xf86-input-neko: DEVICE ON\n");
        device->public.on = TRUE;

        if (priv->thread == 0)
        {
            pthread_create(&priv->thread, NULL, (void *)xf86NekoReadInput, pInfo);
        }
        break;

    case DEVICE_OFF:
    case DEVICE_CLOSE:
        xf86IDrvMsg(pInfo, X_PROBED, "xf86-input-neko: DEVICE OFF\n");
        device->public.on = FALSE;
        break;
    }

    return Success;
}

static int preinit(__attribute__ ((unused)) InputDriverPtr drv,
            InputInfoPtr pInfo,
            __attribute__ ((unused)) int flags)
{
    struct neko_priv *priv;
    char *s;

    priv = calloc(1, sizeof (struct neko_priv));
    if (!priv)
    {
        return BadValue;
    }

    pInfo->type_name = XI_TOUCHSCREEN;
    pInfo->control_proc = NULL;
    pInfo->read_input = NULL;
    pInfo->device_control = xf86NekoControlProc;
    pInfo->switch_mode = NULL;
    pInfo->private = priv;
    pInfo->dev = NULL;
    pInfo->fd = -1;

    s = xf86SetStrOption(pInfo->options, "path", NULL);
    if (!s)
    {
        s = xf86SetStrOption(pInfo->options, "Device", NULL);
    }

    {
        int ret;

        /*
        * In case the program exited inadvertently on the last run,
        * remove the socket.
        */

        unlink(SOCKET_NAME);

        /* Create local socket. */

        priv->listen_socket = socket(AF_UNIX, SOCK_STREAM, 0);
        if (priv->listen_socket == -1)
        {
            perror("socket");
            exit(EXIT_FAILURE);
        }

        /*
        * For portability clear the whole structure, since some
        * implementations have additional (nonstandard) fields in
        * the structure.
        */

        memset(&priv->addr, 0, sizeof(struct sockaddr_un));

        /* Bind socket to socket name. */

        priv->addr.sun_family = AF_UNIX;
        strncpy(priv->addr.sun_path, SOCKET_NAME, sizeof(priv->addr.sun_path) - 1);

        ret = bind(priv->listen_socket, (const struct sockaddr *) &priv->addr,
                sizeof(struct sockaddr_un));
        if (ret == -1)
        {
            perror("bind");
            exit(EXIT_FAILURE);
        }

        /*
        * Prepare for accepting connections. The backlog size is set
        * to 20. So while one request is being processed other requests
        * can be waiting.
        */

        ret = listen(priv->listen_socket, 20);
        if (ret == -1)
        {
            perror("listen");
            exit(EXIT_FAILURE);
        }
    }

    /* process generic options */
    xf86CollectInputOptions(pInfo, NULL);
    xf86ProcessCommonOptions(pInfo, pInfo->options);

    priv->valuators = valuator_mask_new(TOUCH_NUM_AXES);
    if (!priv->valuators)
    {
        return BadValue;
    }

    priv->slots = TOUCH_MAX_SLOTS;
    priv->abs_x_only = 1;
    priv->width = 1024;
    priv->height = 768;
    priv->pmax = 255;
    priv->thread = 0;

    fprintf(stderr, "xf86-input-neko: %s and S is %s\n", __FUNCTION__, s);

    /* Return the configured device */
    return Success;
}

static void uninit(__attribute__ ((unused)) InputDriverPtr drv,
            InputInfoPtr pInfo,
            __attribute__ ((unused)) int flags)
{
    struct neko_priv *priv = (struct neko_priv *)(pInfo->private);

    /* close socket */
    close(priv->listen_socket);
    unlink(SOCKET_NAME);

    if (priv->thread)
    {
        pthread_cancel(priv->thread);
        pthread_join(priv->thread, NULL);
        priv->thread = 0;
    }

    valuator_mask_free(&priv->valuators);
    xf86NekoControlProc(pInfo->dev, DEVICE_OFF);

    free(pInfo->private);
    pInfo->private = NULL;
    xf86DeleteInput(pInfo, 0);
}

_X_EXPORT InputDriverRec NEKO =
{
    .driverVersion = 1,
    .driverName    = "neko",
	.Identify      = NULL,
    .PreInit       = preinit,
    .UnInit        = uninit,
    .module        = NULL
};

static pointer setup(pointer module,
            __attribute__ ((unused)) pointer options,
            __attribute__ ((unused)) int *errmaj,
            __attribute__ ((unused)) int *errmin)
{
    xf86AddInputDriver(&NEKO, module, 0);
    return module;
}

static XF86ModuleVersionInfo vers =
{
	.modname      = "neko",
	.vendor       = MODULEVENDORSTRING,
	._modinfo1_   = MODINFOSTRING1,
	._modinfo2_   = MODINFOSTRING2,
	.xf86version  = XORG_VERSION_CURRENT,
	.majorversion = PACKAGE_VERSION_MAJOR,
	.minorversion = PACKAGE_VERSION_MINOR,
	.patchlevel   = PACKAGE_VERSION_PATCHLEVEL,
	.abiclass     = ABI_CLASS_XINPUT,
	.abiversion   = ABI_XINPUT_VERSION,
	.moduleclass  = MOD_CLASS_XINPUT,
    .checksum     = {0, 0, 0, 0} /* signature, to be patched into the file by a tool */
};

_X_EXPORT XF86ModuleData nekoModuleData =
{
    .vers     = &vers,
    .setup    = setup,
    .teardown = NULL
};
