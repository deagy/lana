@acceptance @local
Feature: Lana local agents and knowledge store
  These scenarios are deterministic regression checks implemented in
  internal/acceptance. They specify the supported local product surface only.

  @SCN-AGT-001
  Scenario: Concurrent workers claim once and preserve a durable update
    Given one queued local agent task in a shared durable store
    When concurrent local workers attempt to claim the task
    Then exactly one worker receives the lease
    And only that worker can persist the terminal task update

  @SCN-AGT-002
  Scenario: Cancellation survives worker loss and recovery
    Given a local worker has claimed a task
    When cancellation is requested and the worker stops before completing
    Then restart recovery records a terminal cancelled task
    And a later completion attempt cannot replace the cancellation

  @SCN-KNO-001
  Scenario: A knowledge index symlink is rejected
    Given a local knowledge-store index path is a symlink
    When Lana reads the local knowledge store
    Then it rejects the path without following the symlink

  @SCN-KNO-002
  Scenario: Human-readable knowledge output is terminal safe
    Given indexed local text contains terminal control or format characters
    When Lana renders a human-readable search result
    Then the output contains no raw terminal control or format character
    And it renders each such character visibly
