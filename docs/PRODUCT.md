# NIMNear — Product

## Overview

NIMNear is a location-based discovery application designed for the Nimiq ecosystem.

Its primary purpose is to help users discover nearby places and experiences through a simple mobile-first interface.

NIMNear is intended to run as a Nimiq Mini App.

## Product Direction

The application should make nearby discovery fast and lightweight.

The user should be able to open the application and quickly understand:
- what is nearby;
- what each place or experience offers;
- where it is;
- why it may be relevant;
- what action can be taken.

## Core Concept

The basic discovery loop is:

User opens NIMNear → location/context becomes available → nearby content is presented → user explores an item → user opens its details → user takes an available action.

Nimiq functionality may be integrated into relevant actions.

## Initial Domain

The expected initial domain includes concepts such as:
- users;
- places;
- nearby discovery;
- experiences/events;
- favorites;
- Nimiq-related actions/payments.

These are product concepts, not finalized database schemas.

Do not create database structures solely from this document.

## Product Principles

### Mobile First
The primary experience should work naturally on mobile devices and inside a Mini App/WebView environment.

### Fast Discovery
Users should not need to navigate through many screens before seeing useful nearby content.

### Location Aware
Location is a central input to discovery. Location permission denial and unavailable location must be handled gracefully.

### Visual Clarity
Important information should be scannable. Avoid unnecessarily dense screens.

### Nimiq Native
Nimiq integration should feel like part of the product rather than a separate cryptocurrency feature.

## MVP Direction

This section is product direction, not a claim that every item is implemented. The current runtime status is recorded below.

The MVP is expected to focus on:
1. discovering nearby places/experiences;
2. browsing discovery results;
3. viewing details;
4. basic user actions such as favorites;
5. Nimiq integration where required by the finalized product flow.

## Current implementation boundary

Implemented backend-backed surfaces include event discovery/detail, active-place discovery/detail, public profiles and profile editing through the legacy JWT, free RSVP, public calendars and legacy-JWT-protected calendar mutations, event creation through the legacy JWT, and paid purchase verification/reconciliation to a configured global merchant.

The normal frontend account-entry UX uses Nimiq Pay listAccounts(), but this is not backend authentication. Protected operations still require the existing JWT. Native Nimiq signature authentication is NO-GO pending the unresolved official Mini App signing contract. Organizer verified recipients, organizer payouts, entitlements, tickets, QR/check-in, refunds, notifications, and city subscription/taxonomy behavior are not implemented.

## Out of Scope Until Explicitly Requested

Do not automatically implement:
- social network features;
- messaging;
- complex recommendation ML;
- admin dashboards;
- large merchant management systems;
- unrelated blockchain functionality;
- Seans Kartı functionality.

Seans Kartı is a separate product concept.

## Source of Truth

Product behavior: this document and explicit product decisions.

Visual behavior: Figma.

Technical behavior: architecture documentation and repository implementation.

When these sources conflict, do not silently guess. Identify the conflict.
