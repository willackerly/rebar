Feature: User Registration

  Scenario: Successful registration
    Given a new user with valid email
    When they submit registration form
    Then account is created
    And confirmation email is sent

  Scenario: Duplicate email
    Given an existing user
    When new user tries to register with same email
    Then registration is rejected
    And error message is shown
